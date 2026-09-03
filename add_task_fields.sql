-- 为 tasks 表添加新字段
-- 用于 Doubao 任务列表筛选功能

-- 1. 添加 token_id 字段（用于按 token 隔离查询）
ALTER TABLE `tasks`
ADD COLUMN `token_id` INT NOT NULL DEFAULT 0 AFTER `user_id`,
ADD INDEX `idx_tasks_token_id` (`token_id`);

-- 2. 添加 model_name 字段（用于按模型筛选）
ALTER TABLE `tasks`
ADD COLUMN `model_name` VARCHAR(100) NOT NULL DEFAULT '' AFTER `channel_id`,
ADD INDEX `idx_tasks_model_name` (`model_name`);

-- 查询验证
SELECT
    COLUMN_NAME,
    COLUMN_TYPE,
    IS_NULLABLE,
    COLUMN_DEFAULT,
    COLUMN_KEY
FROM INFORMATION_SCHEMA.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = 'tasks'
  AND COLUMN_NAME IN ('token_id', 'model_name');
