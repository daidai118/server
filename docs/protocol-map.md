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

### 6.12 `UpdateMapInNpc`

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

### 6.13 `UpdateMapInItem`

- `ItemInfo stInfo`
- `tBOOL bTimeItem`
- `tFLOAT fPosX`
- `tFLOAT fPosZ`
- `tFLOAT fDir`

### 6.14 `UpdateMapOut`

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

## 8. 待抓包确认项

1. 登录、建角、删角、选角各阶段到底哪些命令仍走 `SendNetMessage()`
2. `ReqeustLogin` typed packet 在正式链路中的真实使用点
3. 世界服首包除了 `play` 外是否还存在必须的文本初始化命令
4. SEED 是否全阶段开启，还是只在特定连接阶段启用
