-- 0001 新增 AI run 执行器与 MCP OAuth 网关所需表
-- 幂等：可安全重复执行（IF NOT EXISTS）

CREATE TABLE IF NOT EXISTS `sys_chat_run` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(3) NULL,
  `session_id` BIGINT UNSIGNED NOT NULL,
  `user_id` BIGINT UNSIGNED NOT NULL,
  `status` VARCHAR(20) NOT NULL DEFAULT 'pending',
  `provider_id` BIGINT UNSIGNED NULL,
  `model` VARCHAR(100) NULL,
  `enable_tools` TINYINT(1) DEFAULT 1,
  `question` LONGTEXT NULL,
  `answer` LONGTEXT NULL,
  `tool_events` LONGTEXT NULL,
  `error` LONGTEXT NULL,
  `started_at` DATETIME(3) NULL,
  `finished_at` DATETIME(3) NULL,
  `claimed_by` VARCHAR(64) NULL,
  `claimed_at` DATETIME(3) NULL,
  PRIMARY KEY (`id`),
  KEY `idx_sys_chat_run_status` (`status`),
  KEY `idx_sys_chat_run_user_id` (`user_id`),
  KEY `idx_sys_chat_run_session_id` (`session_id`),
  KEY `idx_sys_chat_run_claimed_by` (`claimed_by`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `sys_oauth_client` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(3) NULL,
  `name` VARCHAR(100) NULL,
  `client_id` VARCHAR(64) NOT NULL,
  `client_secret_hash` VARCHAR(100) NULL,
  `redirect_uris` TEXT NULL,
  `enabled` TINYINT(1) DEFAULT 1,
  `remark` VARCHAR(200) NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_sys_oauth_client_client_id` (`client_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `sys_oauth_code` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(3) NULL,
  `code` VARCHAR(128) NOT NULL,
  `client_id` VARCHAR(64) NULL,
  `user_id` BIGINT UNSIGNED NULL,
  `redirect_uri` TEXT NULL,
  `scope` VARCHAR(100) NULL,
  `code_challenge` VARCHAR(128) NULL,
  `challenge_method` VARCHAR(16) NULL,
  `expires_at` DATETIME(3) NULL,
  `used` TINYINT(1) DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_sys_oauth_code_code` (`code`),
  KEY `idx_sys_oauth_code_client_id` (`client_id`),
  KEY `idx_sys_oauth_code_user_id` (`user_id`),
  KEY `idx_sys_oauth_code_expires_at` (`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS `sys_oauth_refresh_token` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `created_at` DATETIME(3) NULL,
  `token_hash` VARCHAR(64) NOT NULL,
  `client_id` VARCHAR(64) NULL,
  `user_id` BIGINT UNSIGNED NULL,
  `expires_at` DATETIME(3) NULL,
  `revoked` TINYINT(1) DEFAULT 0,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_sys_oauth_refresh_token_token_hash` (`token_hash`),
  KEY `idx_sys_oauth_refresh_token_client_id` (`client_id`),
  KEY `idx_sys_oauth_refresh_token_user_id` (`user_id`),
  KEY `idx_sys_oauth_refresh_token_expires_at` (`expires_at`),
  KEY `idx_sys_oauth_refresh_token_revoked` (`revoked`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;