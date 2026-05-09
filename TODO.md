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

### 协议风险项
- [x] 把 SEED transport 实现补进 Go
- [ ] 用抓包或客户端实机确认 LGS/GMS/ZS 的实际帧形态
- [x] 用客户端源码验证 `ReqeustLogin` 在可见链路中的使用位置
