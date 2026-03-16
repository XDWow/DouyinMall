-- DouyinMall 数据库初始化脚本

-- 创建数据库（如果不存在）
CREATE DATABASE IF NOT EXISTS `douyinmall` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE DATABASE IF NOT EXISTS `douyinmall_agent` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE `douyinmall`;

-- 注：表结构由 GORM AutoMigrate 自动创建
-- 这里可以添加一些初始数据或索引优化

-- 示例：创建管理员账号（可选）
-- INSERT INTO `users` (`email`, `password`, `user_name`, `created_at`, `updated_at`) 
-- VALUES ('admin@douyinmall.com', 'hashed_password', 'admin', NOW(), NOW());

-- 设置时区
SET time_zone = '+08:00';

