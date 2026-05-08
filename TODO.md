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

### P0-3 LGS
- [ ] 确认登录阶段最终 transport：文本命令帧 vs typed packet 的真实组合
- [ ] 实现账号认证服务
- [ ] 对接 SessionManager，签发 GMS ticket
- [ ] 明确错误码与服务器列表结构

### P0-4 GMS
- [ ] ticket 校验中间件
- [ ] 角色列表查询
- [ ] 建角事务（角色 + stats + 默认 inventories）
- [ ] 删角鉴权与软删除
- [ ] 选角后签发 ZS ticket

### P0-5 ZS
- [ ] zone ticket 校验
- [ ] 玩家在线实体创建
- [ ] 基础地图进入
- [ ] `UpdMapInChar / UpdMapInNpc / UpdMapInItem / UpdMapOut`
- [ ] `ReqCharWalk / ReqCharPlace / ReqCharStop` 广播
- [ ] 断线位置落盘

### 协议风险项
- [ ] 把 SEED transport 实现补进 Go
- [ ] 用抓包或客户端实机确认 LGS/GMS/ZS 的实际帧形态
- [ ] 验证 `ReqeustLogin` 在正式链路中的使用位置
