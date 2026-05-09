# TODO

## 已完成

- [x] Task 01: `docs/protocol-map.md`
- [x] Task 02: `docs/database-schema.md`
- [x] Task 03: `docs/session-flow.md`
- [x] Task 04: Go 项目骨架
- [x] Task 05: 协议基础库（帧/包头/opcode 生成/legacy XOR 对照）
- [x] Task 06: MySQL P0 基础 migration
- [x] SessionManager 基础实现与单测

## 下一步（按优先级）

### P0-3 LGS / Gateway
- [x] 确认登录阶段 transport：文本命令帧是主链路，可见路径未使用 typed `ReqLogin`
- [x] 实现账号认证服务
- [x] 对接 SessionManager，建立登录会话并签发逻辑 GMS ticket
- [x] 明确错误码和文本命令响应格式

### P0-4 GMS（逻辑共置于 gateway）
- [x] ticket / handoff 中间件
- [x] 角色列表查询
- [x] 建角流程（角色 + stats + 默认 inventories）
- [x] 删角鉴权与软删除
- [x] 选角后签发 ZS ticket 并登记内部 handoff

### P0-5 ZS
- [x] zone ticket 校验
- [x] 玩家在线实体创建
- [x] 基础地图进入
- [x] `UpdMapInChar / UpdMapInNpc / UpdMapInItem / UpdMapOut`
- [x] `ReqCharWalk / ReqCharPlace / ReqCharStop` 广播
- [x] 断线位置落盘
- [x] 进图 bootstrap 读取真实装备表并下发 `wearing`
- [x] `UpdMapInChar` 带真实装备 wearing vnum
- [x] starter item / quickbar 持久化初始化

### P0-6 物品闭环
- [x] 最小拾取：地面物品进入背包并落库
- [x] 最小丢弃：背包物品移出并广播地面物品
- [x] 最小装备：更新 `equipments` 并广播穿戴变化
- [x] 最小卸下：清理指定装备位并广播脱下变化
- [x] 按客户端源码修正 `ResponseItemPick`：`nInvenPack + nSlotX + nSlotY + ItemInfo`
- [x] 按客户端源码修正 `UpdateItemDrop`：`fPosX + fPosZ + fDir + ItemInfo`
- [x] 按客户端源码修正 `UpdateItemPick`：仅 `nCharIndex`
- [x] 按客户端源码修正 `UpdateCharWear`：`nCharIndex + nWearWhere + nItemVnum + nItemPlusPoint`
- [x] 补上客户端可见文本请求路径：`pick <itemIndex>`、`inven <pack> <x> <y>`、`wear <where>`、`drop 1`
- [ ] 继续抓包确认文本物品链路里 `pick/drop/char_wear/out/pickup` 的完整顺序与失败分支
- [ ] 继续补齐 `quick`、复杂背包换位/交换、事件服装 `ev_wear` 等文本命令
- [ ] 把背包 cursor 从单一 `cursorItemID` 扩展为可恢复的 UI 状态机：拿起、放下、取消、跨 pack、目标格占用
- [ ] 明确物品失败分支回包：背包满、地面物不存在、重复拾取、装备条件失败、空 cursor 丢弃/穿戴
- [ ] 把拾取、移动、装备、建角 starter 初始化收敛到事务边界内，避免 MySQL 半状态
- [ ] 开客户端前补协议/物品分类日志：`gateway / zone / protocol / item`

### 协议风险项
- [x] 把 SEED transport 实现补进 Go
- [ ] 用抓包或客户端实机确认 LGS/GMS/ZS 的实际帧形态
- [x] 用客户端源码验证 `ReqeustLogin` 在可见链路中的使用位置
- [ ] 用抓包确认运行时物品/装备链路到底以文本命令为主，还是 typed packet 与文本混用
- [ ] 用客户端实机确认 `quick`、复杂背包交换、`ev_wear` 的真实发包与回包顺序
