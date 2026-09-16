-- L2 插件预置数据示例
-- 此文件在插件安装时通过事务执行，失败则整体回滚
-- 注意：SQL 语句不能包含事务控制语句（BEGIN/COMMIT/ROLLBACK），因为已在事务中

-- 示例：插入一条系统设置
INSERT INTO sys_setting (key, value, description, is_public, created_at, updated_at)
VALUES ('demo_hello_title', 'Hello from L2 Plugin!', '演示插件预置标题', 1, datetime('now'), datetime('now'));
