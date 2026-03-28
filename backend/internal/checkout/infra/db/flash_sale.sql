-- 秒杀活动表
CREATE TABLE IF NOT EXISTS flash_sale_activities (
    activity_id BIGINT PRIMARY KEY COMMENT '秒杀活动ID',
    product_id BIGINT NOT NULL COMMENT '商品ID',
    flash_price BIGINT NOT NULL COMMENT '秒杀价格（分）',
    total_stock INT NOT NULL COMMENT '秒杀总库存',
    available_stock INT NOT NULL COMMENT '当前可用库存',
    limit_per_user INT NOT NULL DEFAULT 5 COMMENT '每人限购数量',
    start_time BIGINT NOT NULL COMMENT '活动开始时间（毫秒时间戳）',
    end_time BIGINT NOT NULL COMMENT '活动结束时间（毫秒时间戳）',
    is_active BOOLEAN NOT NULL DEFAULT TRUE COMMENT '是否活跃',
    created_at BIGINT NOT NULL COMMENT '创建时间（毫秒时间戳）',
    updated_at BIGINT NOT NULL COMMENT '更新时间（毫秒时间戳）',
    KEY idx_product_id (product_id),
    KEY idx_is_active (is_active)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='秒杀活动表';

-- 秒杀请求表
CREATE TABLE IF NOT EXISTS flash_sale_requests (
    request_id VARCHAR(64) PRIMARY KEY COMMENT '秒杀请求ID（客户端生成的幂等ID）',
    user_id BIGINT NOT NULL COMMENT '用户ID',
    product_id BIGINT NOT NULL COMMENT '商品ID',
    quantity INT NOT NULL COMMENT '购买数量',
    price BIGINT NOT NULL COMMENT '秒杀价格（分）',
    status VARCHAR(32) NOT NULL COMMENT '状态：PENDING/PROCESSING/SUCCESS/FAILED',
    order_id BIGINT DEFAULT 0 COMMENT '订单ID（status=SUCCESS时有值）',
    failure_reason VARCHAR(255) COMMENT '失败原因（status=FAILED时有值）',
    created_at BIGINT NOT NULL COMMENT '创建时间（毫秒时间戳）',
    updated_at BIGINT NOT NULL COMMENT '更新时间（毫秒时间戳）',
    KEY idx_user_id (user_id),
    KEY idx_status (status),
    KEY idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='秒杀请求表';

-- 秒杀库存操作记录表（用于对账和修复）
CREATE TABLE IF NOT EXISTS flash_sale_stock_operations (
    id BIGINT PRIMARY KEY AUTO_INCREMENT COMMENT '主键',
    activity_id BIGINT NOT NULL COMMENT '活动ID',
    operation_type VARCHAR(32) NOT NULL COMMENT '操作类型：RESERVE/COMMIT/RELEASE',
    quantity INT NOT NULL COMMENT '操作数量',
    request_id VARCHAR(64) COMMENT '关联的秒杀请求ID',
    order_id BIGINT COMMENT '关联的订单ID',
    created_at BIGINT NOT NULL COMMENT '创建时间（毫秒时间戳）',
    KEY idx_activity_id (activity_id),
    KEY idx_operation_type (operation_type),
    KEY idx_created_at (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='秒杀库存操作记录表';

