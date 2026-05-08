# MySQL 数据库设计

## 设计原则

1. 主数据与实例数据分离
2. 在线态不直接入库
3. 所有经济变更要有流水
4. 所有 GM 操作要有审计
5. 背包/装备/任务等高频对象要有版本控制字段

## P0 基础实体

```mermaid
erDiagram
    ACCOUNTS ||--o{ CHARACTERS : owns
    CHARACTERS ||--|| CHARACTER_STATS : has
    CHARACTERS ||--o{ INVENTORIES : has
    INVENTORIES ||--o{ INVENTORY_ITEMS : stores
    CHARACTERS ||--o{ EQUIPMENTS : wears
    INVENTORY_ITEMS ||--o| EQUIPMENTS : references
    ACCOUNTS ||--o{ GM_LOGS : writes
    ACCOUNTS ||--o{ GM_LOGS : targets_account
    CHARACTERS ||--o{ GM_LOGS : targets_character
```

## 表清单

### `accounts`

用途：账号主表

关键字段：

- `id`
- `username` 唯一
- `password_hash`
- `password_algo`
- `status`
- `gm_role`
- `last_login_at`
- `last_login_ip`

索引：

- `uk_accounts_username`
- `idx_accounts_status`
- `idx_accounts_role`

### `characters`

用途：角色主表，只放跨系统共享的角色事实

关键字段：

- `id`
- `account_id`
- `slot_index`
- `name` 唯一
- `race / sex / hair`
- `level / experience`
- `map_id / zone_id / pos_x / pos_y / pos_z / direction`
- `money`
- `is_deleted`
- `row_version`

索引：

- `uk_characters_name`
- `uk_characters_account_slot`
- `idx_characters_map`

### `character_stats`

用途：可独立更新的数值块

关键字段：

- 五维属性
- `hp/max_hp`
- `mp/max_mp`
- `stamina/max_stamina`
- `epower/max_epower`
- `skill_points`
- `status_points`
- `row_version`

拆分原因：

- 角色基本信息和数值变化频率不同
- 便于用例层按聚合加载

### `inventories`

用途：容器定义，不直接装物品实例

建议类型：

- `bag`
- `quickbar`
- `stash`
- `guild_stash`

P0 migration 先支持角色自有容器。

### `inventory_items`

用途：背包/仓库里的物品实例

关键字段：

- `inventory_id`
- `slot_index`
- `item_vnum`
- `quantity`
- `plus_point`
- `special_flag_1`
- `special_flag_2`
- `endurance / max_endurance`
- `extra_json`
- `row_version`

索引：

- `uk_inventory_items_slot`
- `idx_inventory_items_item_vnum`

### `equipments`

用途：角色装备位到物品实例的映射

关键字段：

- `character_id`
- `equipment_slot`
- `inventory_item_id`
- `row_version`

约束：

- `PRIMARY KEY (character_id, equipment_slot)`
- `inventory_item_id` 全局唯一，避免一件物品同时被多个装备位引用

### `gm_logs`

用途：后台操作审计

关键字段：

- `operator_account_id`
- `target_account_id`
- `target_character_id`
- `action`
- `reason`
- `request_ip`
- `payload_json`
- `created_at`

## P1/P2 必补表

这些表在设计上已经确定，但 migration 可以分阶段追加：

- `account_bans`
- `skills`
- `quests`
- `quest_progress`
- `stashes`
- `stash_items`
- `guilds`
- `guild_members`
- `friends`
- `friend_requests`
- `shops`
- `shop_logs`
- `trade_logs`
- `item_logs`
- `money_logs`
- `announcements`
- `pets`

## 事务边界建议

### 建角

事务内完成：

1. 插入 `characters`
2. 插入 `character_stats`
3. 创建默认 `inventories`
4. 写初始审计/业务日志（如需要）

### 装备/卸下

事务内完成：

1. 检查物品归属与所在容器
2. 更新 `equipments`
3. 必要时更新 `inventory_items`
4. 递增 `row_version`

### 商店买卖/掉落拾取

事务内完成：

1. 更新角色金钱
2. 更新物品实例
3. 写 `money_logs` / `item_logs`

## 当前已落地 migration

- `migrations/000001_p0_core.up.sql`
- `migrations/000001_p0_core.down.sql`

当前 migration 已覆盖：

- `accounts`
- `characters`
- `character_stats`
- `inventories`
- `inventory_items`
- `equipments`
- `gm_logs`
