# Admin Web 范围

## 目标

P0-P2 期间的后台不是“方便查表的小工具”，而是运营与审计系统。

最低要求：

- 能查账号和角色
- 能做有限 GM 操作
- 所有高风险动作可追责

## 角色模型

| 角色 | 能力 |
|---|---|
| `SuperAdmin` | 所有权限，含管理员管理、RBAC 配置、全局危险操作 |
| `Admin` | 账号/角色管理、公告、日志查看、一般 GM 操作 |
| `GM` | 发物品、修卡图、封禁、广播、查看在线 |
| `Operator` | 运营公告、在线查询、流水查询、只读角色信息 |
| `ReadOnly` | 仅查询，不允许写入 |

## 模块范围

### 1. 账号管理

P0 必做：

- 按账号名 / ID 查询
- 查看封禁状态
- 查看最后登录时间与来源 IP

P1/P2 扩展：

- 封禁 / 解封
- 密码重置
- 权限变更
- 登录历史

### 2. 角色管理

P0 必做：

- 按角色名 / 账号查询
- 查看基础属性、地图、坐标、等级

P1/P2 扩展：

- 修卡图
- 发放物品
- 修改等级 / 经验 / 金币
- 查看背包 / 装备 / 仓库 / 技能 / 任务

### 3. 在线运营

P0 必做：

- 当前在线人数
- 地图人数分布
- 公告发布（只写公告，不做复杂投放）

P2 扩展：

- 按地图踢人
- GM 广播
- 定时公告
- 活动开关

### 4. 审计与流水

从 P0 开始就必须记录：

- GM 操作日志
- 账号封禁变更日志
- 角色修复日志

从 P1 开始补齐：

- 物品流水 `item_logs`
- 金币流水 `money_logs`
- 商店流水 `shop_logs`
- 交易流水 `trade_logs`

## 审计要求

所有后台写操作必须包含：

- `operator_account_id`
- `target_account_id` / `target_character_id`
- `action`
- `reason`
- `request_ip`
- `payload_json`
- `created_at`

不允许匿名 GM 操作。

## API 设计原则

- 读写分离
- 所有写接口默认幂等或可回溯
- 批量危险操作必须二次确认
- 默认分页，禁止无界查询
- 返回结构带审计 ID，方便追踪

## P0 交付清单

### 后台页面

- 登录页
- 账号查询页
- 角色查询页
- 在线人数页
- 公告管理页
- GM 操作日志页

### API

- `POST /api/auth/login`
- `GET /api/accounts`
- `GET /api/accounts/:id`
- `GET /api/characters`
- `GET /api/characters/:id`
- `GET /api/online/summary`
- `GET /api/gm-logs`
- `POST /api/announcements`

## 明确不做

P0 不做：

- 可视化数据编辑器
- 拍卖/邮件后台
- 多租户后台
- 高级 BI 报表

先把“查得准、改得稳、可追责”做出来。
