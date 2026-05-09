# 协议字典（当前基线）

## 证据来源

优先级从高到低：

1. 客户端源码
   - `start_project/common/packet_header.h`
   - `start_project/common/packet_enum.h`
   - `start_project/common/packet_define.h`
   - `start_project/common/info_define.h`
   - `start_project/Laghaim_Client/rnpacket.cpp`
   - `start_project/Laghaim_Client/PacketSend.cpp`
   - `start_project/Laghaim_Client/sheet.cpp`
   - `start_project/Laghaim_Client/Netsock.cpp`
2. 参考仓库逆向结果
   - `/Users/daidai/ai/laghaim-server/README.md`
   - `/Users/daidai/ai/laghaim-server/protocol/__init__.py`
   - `/Users/daidai/ai/laghaim-server/protocol/codec.py`
3. 后续抓包

## 先说结论

当前 Python 参考仓库里，至少有 3 个协议层假设已经和客户端源码冲突：

1. **加密不是单纯 XOR。** 客户端 `rnpacket.cpp` 明确调用 `SeedEncrypt/SeedDecrypt`。
2. **所谓“文本协议”不是裸 TCP 文本。** 客户端 `SendNetMessage()` 会把文本命令包进 `rnPacket` 帧，再走统一加密发送。
3. **Response / Update opcode 不是从 284 往后顺排。** 客户端 `packet_enum.h` 中三组 enum 分别从各自区间起算，`ResLogin=20`，`UpdMapInChar=148` 等。

所以 Go 重写不能直接照抄 Python README 的协议描述。

## 1. 传输层

### 1.1 rnPacket 帧格式

客户端 `packet_header.h` 定义：

```c
struct stPacketHeader {
    unsigned short size_;
};

struct stPacketSubHeader {
    unsigned short index_:14;
    unsigned short type_:2;
    int dummy_;
    unsigned short seq_;
};
```

### 1.2 尺寸语义

这点非常关键。

客户端 `rnpacket.cpp` 中：

- `recalcSize()` 会把 `size_` 设成 `payload_size + PACKET_SUB_HEADER_SIZE`
- 真正发送长度是 `PACKET_HEADER_SIZE + size_`

所以：

| 字段 | 大小 | 含义 |
|---|---:|---|
| `size_` | 2B | **size 字段之后的总字节数**，即 `8B subheader + payload` |
| `subheader` | 8B | opcode/type + dummy + seq |
| `payload` | N | 文本命令或二进制结构体 |

**总帧长 = `2 + size_`**。

### 1.3 `type_` 含义

来自 `packet_header.h` / Python 参考 `protocol/__init__.py`：

| 值 | 含义 |
|---:|---|
| `0` | 文本命令帧常见为全 0 subheader |
| `1` | Request（Client -> Server） |
| `2` | Response（Server -> Client 回复） |
| `3` | Update（Server -> Client 推送） |

## 2. 加密层

### 2.1 客户端已验证结论

客户端 `rnpacket.cpp`：

- `SeedRoundKey(pdwRoundKey, pbUserKey)`
- `SeedEncrypt(...)`
- `SeedDecrypt(...)`

其中 `pbUserKey` 是固定 32 字节：`0x00..0x1f`。

### 2.2 当前仓库中的冲突说明

Python 参考仓库坚持认为：

- 存在 256 字节替换表
- 按私钥 + 公钥 + XOR 处理
- LGS/GMS/ZS 各自不同私钥

这个说法不能直接当成 Go 主实现依据。

### 2.3 当前重构策略

- `internal/protocol/legacyxor/`：保留 Python 逆向笔记的 XOR 公式，仅用于对照/验证旧结论
- 主 transport 以客户端源码为准，后续应补 SEED 实现与抓包验证

## 3. 文本命令帧

客户端 `PacketSend.cpp` / `sheet.cpp` 说明，至少以下命令通过 `SendNetMessage()` 发送：

| 命令 | 来源 | 备注 |
|---|---|---|
| `login\n` | `PacketSend.cpp::SendLogin` | 登录阶段第一行 |
| `play\n` | `PacketSend.cpp::SendLogin` | 世界服重连时使用 |
| `<id>\n` | `PacketSend.cpp::SendLogin` | 第二行 |
| `<pw> <w|d> <f|n> <c|0>\n` | `PacketSend.cpp::SendLogin` | 第三行 |
| `char_new ...\n` | `sheet.cpp::SendNewChar` | 建角命令 |
| `char_del ...\n` | `sheet.cpp::SendDelChar` | 删角命令 |
| `char_exist ...\n` | `sheet.cpp::ConfirmExist` | 名字占用检查 |
| `go_world <world> <sub>\n` | `ControlGate.cpp` | 地图切换/跳转 |

结论：**登录、选角、换图控制命令都不是“纯文本 socket 协议”，而是“文本命令 payload + rnPacket 帧封装”。**

### 3.1 当前客户端可见拓扑

根据 `sheet.cpp::Connect()`、`sheet.cpp::StartGame()`、`main.cpp::ConnectWorld()`，当前客户端可见的连接流程更像：

1. 连接认证/选角入口
2. 发送版本号帧
3. 发送 `login -> username -> password flags`
4. 收到 `chars_*` 文本响应并完成选角
5. 发送 `start ...`
6. 收到 `go_world ip port world area`
7. 断开并连接世界服
8. 再次发送版本号帧
9. 再次发送 `play -> username -> password flags`

也就是说，**客户端源码没有证据表明它会显式携带 GMS/ZS ticket**。因此 ticket/session 应放在服务端内部实现，不应再沿用 Python 参考里“客户端拿 token 直接跳服”的说法。

### 3.2 运行时物品 / 装备控制面：客户端可见路径以文本命令为主

这里是本轮修正后最重要的新结论。

继续追 `ControlInven.cpp` / `NkCharacter.cpp` / `UIMgr.cpp` 可以直接看到，客户端在运行时对物品与装备的可见发送路径，至少包括：

| 命令 | 来源 | 作用 |
|---|---|---|
| `pick <itemIndex>\n` | `UIMgr.cpp` | 拾取地面物品 |
| `inven <pack> <x> <y>\n` | `ControlInven.cpp` / `NkCharacter.cpp` | 背包格子选取/放置/移动 |
| `wear <where>\n` | `ControlInven.cpp` / `NkCharacter.cpp` | 穿戴 / 卸下普通装备 |
| `ev_wear <where>\n` | `ControlInven.cpp` | 事件服装穿戴 |
| `drop 1\n` | `ControlInven.cpp` / `Nk2DFrame.cpp` | 丢弃当前“额外槽/光标”中的物品 |
| `quick <slot>\n` | `ControlBottom.cpp` | 快捷栏变更 |

这意味着：

1. `packet_define.h` 里的 `ReqWear / ReqInven / ReqItemPick / ReqItemDrop` **不能再直接当成 P0 主链路依据**。
2. 真实客户端很可能是 **文本命令驱动 UI 同步**，typed packet 只保留给部分旧路径、旁路功能或历史实现。
3. 服务器至少要支持 `pick / inven / wear / drop` 这组文本命令，否则真实客户端运行时物品链路会直接卡住。

本仓库当前状态：

- **已实现**：世界服文本 `pick / inven / wear / drop` 最小闭环；typed `ResponseItemPick / UpdateItemDrop / UpdateItemPick / UpdateCharWear` 也已按客户端 struct 校准，便于继续抓包比对。
- **仍未完成**：`quick`、复杂背包换位/交换、`ev_wear`、以及文本链路失败分支的完整对齐。

## 4. 客户端 opcode 计数

来自 `packet_enum.h`，并已用 `tools/gen_opcodes.py` 生成到 Go 代码：

- Request: **286** 个
- Response: **201** 个
- Update: **235** 个

生成文件：

- `internal/protocol/opcodes_gen.go`

## 5. P0 关键 opcode

### 5.1 Request

| 名称 | 值 | 用途 |
|---|---:|---|
| `ReqLogin` | 27 | 登录相关 typed packet（客户端源码有定义，实际链路需结合文本命令再确认） |
| `ReqCharNew` | 28 | 建角 |
| `ReqCharDel` | 29 | 删角 |
| `ReqGameStart` | 30 | 角色选择完成 |
| `ReqGamePlayReady` | 31 | 场景加载完成 |
| `ReqWear` | 32 | 装备/卸下 |
| `ReqInven` | 34 | 背包变更 |
| `ReqQuick` | 35 | 快捷栏 |
| `ReqPulse` | 36 | 心跳/测速相关 |
| `ReqCharWalk` | 37 | 移动 |
| `ReqCharPlace` | 38 | 强制定位/校正 |
| `ReqCharStop` | 39 | 停止 |
| `ReqCharGoto` | 40 | 指向性移动 |
| `ReqGoWorld` | 43 | 换图/换世界 |
| `ReqItemDrop` | 76 | 丢地面 |
| `ReqItemPick` | 77 | 拾取 |

### 5.2 Response

| 名称 | 值 | 用途 |
|---|---:|---|
| `ResLogin` | 20 | 登录结果 |
| `ResCharNew` | 21 | 建角结果 |
| `ResCharDel` | 22 | 删角结果 |
| `ResGameStart` | 23 | 角色选择结果 |
| `ResGamePlayReady` | 24 | 进入世界准备结果 |
| `ResCharGoto` | 25 | 移动/定位响应 |
| `ResGoWorld` | 26 | 换图结果 |
| `ResItemPick` | 52 | 拾取结果 |
| `ResItemShopList` | 59 | 商店列表 |

### 5.3 Update

| 名称 | 值 | 用途 |
|---|---:|---|
| `UpdInfoMessage` | 40 | 提示消息 |
| `UpdCharWalk` | 44 | 玩家/NPC 行走广播 |
| `UpdCharPlace` | 45 | 定位广播 |
| `UpdCharStop` | 46 | 停止广播 |
| `UpdCharAttack` | 57 | 攻击广播 |
| `UpdItemDrop` | 64 | 地面掉落 |
| `UpdItemPick` | 65 | 拾取广播 |
| `UpdCharStatus` | 16 | 状态变化 |
| `UpdCharLevelUp` | 68 | 升级 |
| `UpdCharRevive` | 70 | 复活 |
| `UpdMapInChar` | 148 | 玩家进入可见范围 |
| `UpdMapInNpc` | 149 | NPC 进入可见范围 |
| `UpdMapInItem` | 151 | 地面物品进入可见范围 |
| `UpdMapOut` | 152 | 对象离开可见范围 |
| `UpdChatNormal` | 231 | 普通聊天 |
| `UpdChatTell` | 232 | 私聊 |
| `UpdChatGuild` | 233 | 公会聊天 |
| `UpdChatParty` | 234 | 组队聊天 |

## 6. P0 关键包体定义

说明：以下长度均基于 `#pragma pack(push, 1)`，`tINT=4B`，`tFLOAT=4B`，`tBOOL=1B`。

### 6.1 `ReqeustLogin`

来源：`packet_define.h`

| 顺序 | 字段 | 类型 | 大小 |
|---:|---|---|---:|
| 1 | `nPackVersion` | `tINT` | 4 |
| 2 | `szId` | `tCHAR[51]` | 51 |
| 3 | `szPw` | `tCHAR[21]` | 21 |
| 4 | `bWeb` | `tBOOL` | 1 |
| 5 | `bFirm` | `tBOOL` | 1 |
| 6 | `bInternalConnect` | `tBOOL` | 1 |

补充结论：

- 在当前客户端源码里，`ReqLogin / ReqGameStart / ReqGamePlayReady` 只在 `packet_enum.h` 中出现
- 没有检索到对应的发送代码路径
- 当前可见登录/选角/进图链路实际走的是 `SendNetMessage()` 文本命令帧

所以在 P0 实现中，`ReqeustLogin` 应视为**待进一步抓包确认的备用/遗留结构**，而不是主链路首选实现。

### 6.2 `PreCharInfo` / `RequestCharNew`

来源：`info_define.h` + `packet_define.h`

`RequestCharNew` 仅包一层：

- `stInfo PreCharInfo`

`PreCharInfo` 关键字段：

| 顺序 | 字段 | 类型 | 大小 |
|---:|---|---|---:|
| 1 | `nSlotIndex` | `tINT` | 4 |
| 2 | `nCharIndex` | `tINT` | 4 |
| 3 | `szId` | `tCHAR[50]` | 50 |
| 4 | `nRace` | `tINT` | 4 |
| 5 | `nSex` | `tINT` | 4 |
| 6 | `nHair` | `tINT` | 4 |
| 7-19 | 等级/HP/MP/体力/属性等 | `tINT` | 13 × 4 |
| 20 | `nGuildIndex` | `tINT` | 4 |
| 21 | `nWearing[6]` | `tINT[6]` | 24 |
| 22 | `bIsGuildMaster` | `tBOOL` | 1 |
| 23 | `bIsSupport` | `tBOOL` | 1 |

### 6.3 `RequestCharDel`

| 顺序 | 字段 | 类型 | 大小 |
|---:|---|---|---:|
| 1 | `nSlotIndex` | `tINT` | 4 |
| 2 | `nCharIndex` | `tINT` | 4 |
| 3 | `nGuildIndex` | `tINT` | 4 |
| 4 | `bWeb` | `tBOOL` | 1 |
| 5 | `szPw_length` | `tCHAR` | 1 |
| 6 | `szPw` | `tCHAR[0]` | 可变 |

### 6.4 `RequestGameStart`

| 顺序 | 字段 | 类型 | 大小 |
|---:|---|---|---:|
| 1 | `nSlotIndex` | `tINT` | 4 |
| 2 | `nCharCount` | `tINT` | 4 |
| 3 | `bSlotEmpty[5]` | `tBOOL[5]` | 5 |

### 6.5 `RequestPulse`

| 顺序 | 字段 | 类型 | 大小 |
|---:|---|---|---:|
| 1 | `nClientTime` | `tINT` | 4 |
| 2 | `nItemVnum` | `tINT` | 4 |
| 3 | `nItemIndex` | `tINT` | 4 |
| 4 | `nItemSpeed` | `tINT` | 4 |
| 5 | `nItemAttackRange` | `tINT` | 4 |

### 6.6 `RequestCharWalk`

| 顺序 | 字段 | 类型 | 大小 |
|---:|---|---|---:|
| 1 | `fPosX` | `tFLOAT` | 4 |
| 2 | `fPosZ` | `tFLOAT` | 4 |
| 3 | `bRun` | `tBOOL` | 1 |

### 6.7 `RequestCharPlace`

| 顺序 | 字段 | 类型 | 大小 |
|---:|---|---|---:|
| 1 | `btCharType` | `tBYTE` | 1 |
| 2 | `fPosX` | `tFLOAT` | 4 |
| 3 | `fPosZ` | `tFLOAT` | 4 |
| 4 | `fDir` | `tFLOAT` | 4 |
| 5 | `nRemainFrame` | `tINT` | 4 |
| 6 | `bRun` | `tBOOL` | 1 |

### 6.8 `RequestCharStop`

| 顺序 | 字段 | 类型 | 大小 |
|---:|---|---|---:|
| 1 | `btCharType` | `tBYTE` | 1 |
| 2 | `fPosX` | `tFLOAT` | 4 |
| 3 | `fPosZ` | `tFLOAT` | 4 |
| 4 | `fDir` | `tFLOAT` | 4 |

### 6.9 `RequestCharGoto`

| 顺序 | 字段 | 类型 | 大小 |
|---:|---|---|---:|
| 1 | `btCharType` | `tBYTE` | 1 |
| 2 | `fPosX` | `tFLOAT` | 4 |
| 3 | `fPosZ` | `tFLOAT` | 4 |

### 6.10 `UpdateCharWalk`

| 顺序 | 字段 | 类型 | 大小 |
|---:|---|---|---:|
| 1 | `btCharType` | `tBYTE` | 1 |
| 2 | `nTargetIndex` | `tINT` | 4 |
| 3 | `fPosX` | `tFLOAT` | 4 |
| 4 | `fPosZ` | `tFLOAT` | 4 |
| 5 | `bRun` | `tBOOL` | 1 |

### 6.11 `UpdateMapInChar`

关键字段顺序：

1. `btType`
2. `nCharIndex`
3. `nCharName[50]`
4. `nCharRace`
5. `nCharSex`
6. `nCharHair`
7. `fPosX`
8. `fPosZ`
9. `fDir`
10. `nItemVnum[7]`
11. `nVital`
12. `nCombatState`
13. `nSkillIndex`
14. `nState`
15. `nSubSkill`
16. `nChaoGrade`
17. `nPK`
18. `nSky`
19. `strGuildName[30]`
20. `strGuildGrade[30]`
21. `nGuildLevel`
22. `nGuildIndex`
23. `nGuildType`

### 6.12 物品 / 装备协议状态（本轮已校准部分 + 仍存风险）

以下结论来自客户端源码：

- `start_project/common/packet_define.h`
- `start_project/common/info_define.h`
- `start_project/common/packet_enum.h`
- `start_project/Laghaim_Client/ControlInven.cpp`
- `start_project/Laghaim_Client/NkCharacter.cpp`
- `start_project/Laghaim_Client/UIMgr.cpp`

#### 6.12.1 typed struct 校准结果

| 包体 | 客户端源码定义 | 当前 Go 实现状态 |
|---|---|---|
| `RequestWear` | `tINT nWearWhere` | **已修正**，Go 现按单个 `nWearWhere` 解码 |
| `RequestItemPick` | `tINT nItemIndex` | 与源码一致 |
| `ResponseItemPick` | `tINT nInvenPack; tINT nSlotX; tINT nSlotY; ItemInfo stInfo` | **已修正** |
| `UpdateCharWear` | `tINT nCharIndex; tINT nWearWhere; tINT nItemVnum; tINT nItemPlusPoint` | **已修正** |
| `UpdateItemDrop` | `tFLOAT fPosX; tFLOAT fPosZ; tFLOAT fDir; ItemInfo stInfo` | **已修正** |
| `UpdateItemPick` | `tINT nCharIndex` | **已修正** |
| `UpdateMapInItem` | `ItemInfo stInfo; tBOOL bTimeItem; tFLOAT fPosX; tFLOAT fPosZ; tFLOAT fDir` | 与源码一致 |
| `ItemInfo` | 9 个 `tINT`：`nItemIndex/nItemVnum/nPlusPoint/nSpecialFlag1/nSpecialFlag2/nUpgradeEndurance/nUpgradeEnduranceMax/nEndurance/nEnduranceMax` | 与源码一致 |

#### 6.12.2 客户端可见文本链路

继续追 UI 发送代码后，当前更可信的结论是：**运行时物品 / 装备主链路以文本命令为主，typed struct 主要用于校准和抓包比对。**

当前 Go 已补上的客户端可见文本请求：

- `pick <itemIndex>`
- `inven <pack> <x> <y>`
- `wear <where>`
- `drop 1`

当前 Go 已补上的客户端可见文本响应 / 广播：

- `pick ...`
- `drop ...`
- `out i <itemIndex>`
- `pickup <charIndex>`
- `char_wear <charIndex> <where> <vnum> <plus>`
- `char_remove <charIndex> <where>`

#### 6.12.3 仍未确认 / 仍有风险的点

- `quick <slot>` 运行时写链路还没补，当前只做了 bootstrap 下发。
- 背包复杂交换（目标格已有物品时的完整 swap 语义）还没有完全按客户端 UI 状态机补齐。
- `ev_wear <where>` 事件服装仍未实现。
- typed `ResponseItemPick / UpdateItemDrop / UpdateItemPick / UpdateCharWear` 虽已校准，但**真实线上链路是否还会下发这些 typed 包**，仍需抓包确认。
- `pick` 成功后客户端到底依赖 `pick` 文本、typed `ResponseItemPick`、还是两者混用，目前还没有抓包闭环证据；当前 Go 走的是“文本主链路 + typed struct 校准保留”的策略。

客户端实测时重点抓：

1. `pick` 成功时服务端真实下发顺序：`out` / `pick` / `pickup` / typed `ResItemPick` 是否混用。
2. `wear` / `drop` / `inven` 在成功、失败、背包满、目标格占用时的真实返回序列。
3. `char_wear` / `char_remove` 是否足够让其他玩家刷新模型，自己是否还需要 `UpdateMyCharWearing` 或其他文本命令。
4. `quick`、复杂背包换位、事件服装 `ev_wear` 的真实协议形态。
5. `ReqItemDrop` / `ReqInven` / `ReqWear` 这些 typed opcode 在正式链路里是否还有残留使用点。

### 6.13 `UpdateMapInNpc`

关键字段顺序：

1. `nNpcIndex`
2. `nNpcVnum`
3. `fPosX`
4. `fPosZ`
5. `fDir`
6. `nMobEventFlag`
7. `nVital`
8. `nMutant`
9. `nAttribute`
10. `nMobClass`
11. `nState`
12. `nQuestLevel`
13. `nRegen`

### 6.14 `UpdateMapInItem`

- `ItemInfo stInfo`
- `tBOOL bTimeItem`
- `tFLOAT fPosX`
- `tFLOAT fPosZ`
- `tFLOAT fDir`

### 6.15 `UpdateMapOut`

| 顺序 | 字段 | 类型 | 大小 |
|---:|---|---|---:|
| 1 | `btCharType` | `tBYTE` | 1 |
| 2 | `nCharIndex` | `tINT` | 4 |

## 7. 当前 Go 基础实现对应关系

已落地：

- `internal/protocol/header.go`
- `internal/protocol/frame.go`
- `tools/gen_opcodes.py`
- `internal/protocol/opcodes_gen.go`
- `internal/protocol/legacyxor/`

当前已验证能力：

- 14-bit opcode + 2-bit type 的 subheader 编解码
- 按客户端 `size_` 语义构建/切分帧
- 文本命令帧封装
- 基于客户端 `SEED_256_KISA.cpp` 的 SEED transport（已用参考向量校验）
- 从客户端 `packet_enum.h` 生成 Go opcode 常量
- 世界服文本 `pick / inven / wear / drop` 最小闭环
- typed `ResponseItemPick / UpdateItemDrop / UpdateItemPick / UpdateCharWear` 已按客户端 struct 校准
- `go_world` 已支持独立 `advertise_host / advertise_port`，避免部署时把 `0.0.0.0` 错发给客户端

## 8. 待抓包确认项

1. 登录、建角、删角、选角各阶段到底哪些命令仍走 `SendNetMessage()`。
2. `ReqeustLogin` typed packet 在正式链路中的真实使用点。
3. 世界服首包除了 `play` 外是否还存在必须的文本初始化命令。
4. 运行时物品 / 装备链路里，文本 `pick / inven / wear / drop` 与 typed `ResItemPick / UpdItemDrop / UpdItemPick / UpdCharWear` 的真实混用比例。
5. `quick`、复杂背包交换、`ev_wear` 的真实发包与回包顺序。
6. `out i <itemIndex>` / `pickup <charIndex>` / `char_wear` / `char_remove` 的广播顺序与失败分支。
7. SEED 是否全阶段开启，还是只在特定连接阶段启用。

## 9. 本轮新增风险点

### 高风险

1. **文本链路与 typed 链路混用仍未抓包定论**
   当前最小闭环同时保留了客户端可见文本命令和 typed Response/Update 校准包。没有抓包前，不能断言真实客户端只依赖其中一条路径。开客户端后要逐包确认 `pick / inven / wear / drop` 与 `ResItemPick / UpdItemDrop / UpdItemPick / UpdCharWear` 的真实顺序。

2. **背包 cursor 状态机仍是简化版**
   当前 Zone Server 用 `state.cursorItemID` 表示“当前拿起的物品”。这能支持单物品 pick/place/wear/drop，但真实客户端可能存在 extra slot、交换、取消拖拽、跨 pack 拖拽、目标格已有物品、窗口关闭等状态。如果客户端 UI 状态和服务端 cursor 不一致，后续 `wear/drop/inven` 会操作错物品。

3. **物品操作事务边界不完整**
   MySQL 里建角流程、拾取、移动、装备大多是多条 SQL 分散执行，只有删除物品时显式事务。高并发或异常断开时可能出现“地面物品已移除但背包创建失败”“装备映射更新成功但背包移动失败”等半状态。P0 可接受，实测前要知道这是风险。

4. **真实客户端回包失败分支未知**
   当前很多失败分支直接静默返回，或者只保留成功路径。真实客户端可能需要明确失败文本、typed result、或本地回滚命令。特别是背包满、目标格占用、装备条件不满足、地面物品不存在这些场景，需要实机确认。

### 中风险

5. **`quick` / `ev_wear` 仍缺运行时实现**
   `quick` 目前只有 bootstrap 下发，运行时写链路还没补；事件服装 `ev_wear` 也未实现。客户端如果在进图后拖拽快捷栏或事件装备，会出现 UI 本地状态与服务端持久态不一致。

6. **在线地面物品只存在内存**
   `groundItems` 是 Zone Server 内存 map。服务重启会丢地面物品，且多 Zone 进程无法共享。P0 单进程没问题，但不能直接当正式掉落系统。

7. **AOI 与广播范围过粗**
   当前按 `map_id + zone_id` 全量广播，没有距离、格子、视野半径。实机少量玩家能跑，人数一多会有性能和可见性误差。

8. **部署入口风险**
   当前真正可部署的是 `cmd/dev-cluster`；单独 `cmd/login-server` / `cmd/zone-server` 还只是 bootstrap 占位，不是完整分布式拓扑。

9. **地址通告风险**
   如果部署时没有设置 `zone_server.advertise_host / advertise_port`，`go_world` 可能把 `0.0.0.0`、内网地址或错误端口发给客户端。

### 低风险 / 运维风险

10. **日志追溯能力不足**
    当前仓库只有基础 stdout 结构化日志，还没有按 `gateway / zone / protocol / item / audit` 分类单独落盘。开客户端抓包联调前，建议把日志分层补上，否则定位时会很痛苦。

11. **MySQL smoke 覆盖仍需真实环境确认**
    仓库已有 MySQL repo 和脚本，但本地/CI 是否真的跑过 migration + smoke 取决于环境。尤其要确认 MariaDB/MySQL 版本、生成列、JSON 字段、`ON DUPLICATE KEY` 行为一致。

12. **协议文档编号和旧结论需要持续清理**
    随着文本链路被确认，部分早期 typed 假设会过期。每次实机确认后都应同步删掉旧假设，避免后续实现者误读。
