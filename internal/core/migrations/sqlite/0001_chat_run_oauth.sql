-- 0001 新增 AI run 执行器与 MCP OAuth 网关所需表
-- 幂等：可安全重复执行（IF NOT EXISTS）

CREATE TABLE IF NOT EXISTS `sys_chat_run` (
  `id` integer PRIMARY KEY AUTOINCREMENT,
  `created_at` datetime,
  `session_id` integer NOT NULL,
  `user_id` integer NOT NULL,
  `status` text NOT NULL DEFAULT 'pending',
  `provider_id` integer,
  `model` text,
  `enable_tools` numeric DEFAULT true,
  `question` text,
  `answer` text,
  `tool_events` text,
  `error` text,
  `started_at` datetime,
  `finished_at` datetime,
  `claimed_by` text,
  `claimed_at` datetime
);
CREATE INDEX IF NOT EXISTS `idx_sys_chat_run_status` ON `sys_chat_run`(`status`);
CREATE INDEX IF NOT EXISTS `idx_sys_chat_run_user_id` ON `sys_chat_run`(`user_id`);
CREATE INDEX IF NOT EXISTS `idx_sys_chat_run_session_id` ON `sys_chat_run`(`session_id`);
CREATE INDEX IF NOT EXISTS `idx_sys_chat_run_claimed_by` ON `sys_chat_run`(`claimed_by`);

CREATE TABLE IF NOT EXISTS `sys_oauth_client` (
  `id` integer PRIMARY KEY AUTOINCREMENT,
  `created_at` datetime,
  `name` text,
  `client_id` text NOT NULL,
  `client_secret_hash` text,
  `redirect_uris` text,
  `enabled` numeric DEFAULT true,
  `remark` text
);
CREATE UNIQUE INDEX IF NOT EXISTS `idx_sys_oauth_client_client_id` ON `sys_oauth_client`(`client_id`);

CREATE TABLE IF NOT EXISTS `sys_oauth_code` (
  `id` integer PRIMARY KEY AUTOINCREMENT,
  `created_at` datetime,
  `code` text NOT NULL,
  `client_id` text,
  `user_id` integer,
  `redirect_uri` text,
  `scope` text,
  `code_challenge` text,
  `challenge_method` text,
  `expires_at` datetime,
  `used` numeric DEFAULT false
);
CREATE UNIQUE INDEX IF NOT EXISTS `idx_sys_oauth_code_code` ON `sys_oauth_code`(`code`);
CREATE INDEX IF NOT EXISTS `idx_sys_oauth_code_client_id` ON `sys_oauth_code`(`client_id`);
CREATE INDEX IF NOT EXISTS `idx_sys_oauth_code_user_id` ON `sys_oauth_code`(`user_id`);
CREATE INDEX IF NOT EXISTS `idx_sys_oauth_code_expires_at` ON `sys_oauth_code`(`expires_at`);

CREATE TABLE IF NOT EXISTS `sys_oauth_refresh_token` (
  `id` integer PRIMARY KEY AUTOINCREMENT,
  `created_at` datetime,
  `token_hash` text NOT NULL,
  `client_id` text,
  `user_id` integer,
  `expires_at` datetime,
  `revoked` numeric DEFAULT false
);
CREATE UNIQUE INDEX IF NOT EXISTS `idx_sys_oauth_refresh_token_token_hash` ON `sys_oauth_refresh_token`(`token_hash`);
CREATE INDEX IF NOT EXISTS `idx_sys_oauth_refresh_token_client_id` ON `sys_oauth_refresh_token`(`client_id`);
CREATE INDEX IF NOT EXISTS `idx_sys_oauth_refresh_token_user_id` ON `sys_oauth_refresh_token`(`user_id`);
CREATE INDEX IF NOT EXISTS `idx_sys_oauth_refresh_token_expires_at` ON `sys_oauth_refresh_token`(`expires_at`);
CREATE INDEX IF NOT EXISTS `idx_sys_oauth_refresh_token_revoked` ON `sys_oauth_refresh_token`(`revoked`);