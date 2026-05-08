CREATE TABLE accounts (
    id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    username          VARCHAR(50)     NOT NULL,
    password_hash     VARBINARY(255)  NOT NULL,
    password_algo     VARCHAR(32)     NOT NULL DEFAULT 'argon2id',
    status            VARCHAR(16)     NOT NULL DEFAULT 'active',
    gm_role           VARCHAR(16)     NOT NULL DEFAULT 'player',
    last_login_at     DATETIME(6)     NULL,
    last_login_ip     VARCHAR(45)     NULL,
    created_at        DATETIME(6)     NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at        DATETIME(6)     NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uk_accounts_username (username),
    KEY idx_accounts_status (status),
    KEY idx_accounts_role (gm_role)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE characters (
    id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    account_id        BIGINT UNSIGNED NOT NULL,
    slot_index        TINYINT UNSIGNED NOT NULL,
    name              VARCHAR(50)     NOT NULL,
    race              TINYINT UNSIGNED NOT NULL,
    sex               TINYINT UNSIGNED NOT NULL,
    hair              TINYINT UNSIGNED NOT NULL,
    level             INT UNSIGNED    NOT NULL DEFAULT 1,
    experience        BIGINT UNSIGNED NOT NULL DEFAULT 0,
    map_id            INT UNSIGNED    NOT NULL DEFAULT 1,
    zone_id           INT UNSIGNED    NOT NULL DEFAULT 0,
    pos_x             DECIMAL(10,3)   NOT NULL DEFAULT 33000.000,
    pos_y             DECIMAL(10,3)   NOT NULL DEFAULT 0.000,
    pos_z             DECIMAL(10,3)   NOT NULL DEFAULT 33000.000,
    direction         DECIMAL(10,3)   NOT NULL DEFAULT 0.000,
    money             BIGINT UNSIGNED NOT NULL DEFAULT 0,
    is_deleted        TINYINT(1)      NOT NULL DEFAULT 0,
    row_version       BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at        DATETIME(6)     NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at        DATETIME(6)     NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    CONSTRAINT fk_characters_account FOREIGN KEY (account_id) REFERENCES accounts (id),
    UNIQUE KEY uk_characters_name (name),
    UNIQUE KEY uk_characters_account_slot (account_id, slot_index),
    KEY idx_characters_account (account_id),
    KEY idx_characters_map (map_id, zone_id),
    KEY idx_characters_deleted (account_id, is_deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE character_stats (
    character_id       BIGINT UNSIGNED NOT NULL,
    strength           INT UNSIGNED    NOT NULL,
    intelligence       INT UNSIGNED    NOT NULL,
    dexterity          INT UNSIGNED    NOT NULL,
    constitution       INT UNSIGNED    NOT NULL,
    charisma           INT UNSIGNED    NOT NULL,
    hp                 INT UNSIGNED    NOT NULL,
    max_hp             INT UNSIGNED    NOT NULL,
    mp                 INT UNSIGNED    NOT NULL,
    max_mp             INT UNSIGNED    NOT NULL,
    stamina            INT UNSIGNED    NOT NULL DEFAULT 0,
    max_stamina        INT UNSIGNED    NOT NULL DEFAULT 0,
    epower             INT UNSIGNED    NOT NULL DEFAULT 0,
    max_epower         INT UNSIGNED    NOT NULL DEFAULT 0,
    skill_points       INT UNSIGNED    NOT NULL DEFAULT 0,
    status_points      INT UNSIGNED    NOT NULL DEFAULT 0,
    row_version        BIGINT UNSIGNED NOT NULL DEFAULT 1,
    updated_at         DATETIME(6)     NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (character_id),
    CONSTRAINT fk_character_stats_character FOREIGN KEY (character_id) REFERENCES characters (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE inventories (
    id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    character_id      BIGINT UNSIGNED NOT NULL,
    inventory_type    VARCHAR(16)     NOT NULL,
    capacity          INT UNSIGNED    NOT NULL,
    row_version       BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at        DATETIME(6)     NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at        DATETIME(6)     NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    CONSTRAINT fk_inventories_character FOREIGN KEY (character_id) REFERENCES characters (id),
    UNIQUE KEY uk_inventories_character_type (character_id, inventory_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE inventory_items (
    id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    inventory_id      BIGINT UNSIGNED NOT NULL,
    slot_index        INT UNSIGNED    NOT NULL,
    item_vnum         INT UNSIGNED    NOT NULL,
    quantity          INT UNSIGNED    NOT NULL DEFAULT 1,
    plus_point        INT             NOT NULL DEFAULT 0,
    special_flag_1    INT             NOT NULL DEFAULT 0,
    special_flag_2    INT             NOT NULL DEFAULT 0,
    endurance         INT             NOT NULL DEFAULT 0,
    max_endurance     INT             NOT NULL DEFAULT 0,
    extra_json        JSON            NULL,
    row_version       BIGINT UNSIGNED NOT NULL DEFAULT 1,
    created_at        DATETIME(6)     NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at        DATETIME(6)     NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    CONSTRAINT fk_inventory_items_inventory FOREIGN KEY (inventory_id) REFERENCES inventories (id),
    UNIQUE KEY uk_inventory_items_slot (inventory_id, slot_index),
    KEY idx_inventory_items_item_vnum (item_vnum)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE equipments (
    character_id      BIGINT UNSIGNED NOT NULL,
    equipment_slot    TINYINT UNSIGNED NOT NULL,
    inventory_item_id BIGINT UNSIGNED NOT NULL,
    row_version       BIGINT UNSIGNED NOT NULL DEFAULT 1,
    updated_at        DATETIME(6)     NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (character_id, equipment_slot),
    CONSTRAINT fk_equipments_character FOREIGN KEY (character_id) REFERENCES characters (id),
    CONSTRAINT fk_equipments_item FOREIGN KEY (inventory_item_id) REFERENCES inventory_items (id),
    UNIQUE KEY uk_equipments_item (inventory_item_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE gm_logs (
    id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    operator_account_id BIGINT UNSIGNED NOT NULL,
    target_account_id BIGINT UNSIGNED NULL,
    target_character_id BIGINT UNSIGNED NULL,
    action            VARCHAR(64)     NOT NULL,
    reason            VARCHAR(255)    NULL,
    request_ip        VARCHAR(45)     NULL,
    payload_json      JSON            NULL,
    created_at        DATETIME(6)     NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    CONSTRAINT fk_gm_logs_operator FOREIGN KEY (operator_account_id) REFERENCES accounts (id),
    CONSTRAINT fk_gm_logs_target_account FOREIGN KEY (target_account_id) REFERENCES accounts (id),
    CONSTRAINT fk_gm_logs_target_character FOREIGN KEY (target_character_id) REFERENCES characters (id),
    KEY idx_gm_logs_action_created (action, created_at),
    KEY idx_gm_logs_target_character (target_character_id, created_at),
    KEY idx_gm_logs_target_account (target_account_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
