-- ====================
-- 优惠券服务数据库设计
-- ====================

-- 1. 优惠券模板表（商家/运营创建）
CREATE TABLE coupon_templates (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    
    -- 基础信息
    name VARCHAR(128) NOT NULL COMMENT '优惠券名称',
    description VARCHAR(512) DEFAULT '' COMMENT '描述',
    
    -- 类型: 1-满减 2-折扣 3-固定金额
    coupon_type TINYINT UNSIGNED NOT NULL DEFAULT 1,
    
    -- 优惠券规则（单位：分）
    discount_value INT NOT NULL COMMENT '折扣值：满减/固定金额时为分，折扣时为百分比(85=8.5折)',
    min_order_amount INT NOT NULL DEFAULT 0 COMMENT '最低订单金额（分）',
    max_discount_amount INT DEFAULT NULL COMMENT '最大折扣金额（分），折扣券时有效',
    
    -- 适用范围（空=全品类）
    applicable_product_ids JSON DEFAULT NULL COMMENT '适用商品ID列表',
    applicable_category_ids JSON DEFAULT NULL COMMENT '适用品类ID列表',
    
    -- 有效期设置
    valid_type TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '1-固定时间 2-领取后N天',
    valid_start_time DATETIME DEFAULT NULL COMMENT '有效开始时间（固定时间类型）',
    valid_end_time DATETIME DEFAULT NULL COMMENT '有效结束时间（固定时间类型）',
    valid_days INT DEFAULT NULL COMMENT '领取后有效天数（N天类型）',
    
    -- 发放控制
    total_count INT NOT NULL DEFAULT 0 COMMENT '总发放数量，0=无限制',
    issued_count INT NOT NULL DEFAULT 0 COMMENT '已发放数量',
    per_user_limit INT NOT NULL DEFAULT 1 COMMENT '每人限领数量',
    
    -- 状态: 1-启用 2-禁用
    status TINYINT UNSIGNED NOT NULL DEFAULT 1,
    
    -- 商家信息（如果是店铺券）
    merchant_id BIGINT UNSIGNED DEFAULT NULL COMMENT '商家ID，NULL表示平台券',
    
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    INDEX idx_merchant_id (merchant_id),
    INDEX idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='优惠券模板表';


-- 2. 用户优惠券表
CREATE TABLE user_coupons (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    
    user_id BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
    template_id BIGINT UNSIGNED NOT NULL COMMENT '优惠券模板ID',
    
    -- 状态: 1-未使用 2-已锁定 3-已使用 4-已退还
    status TINYINT UNSIGNED NOT NULL DEFAULT 1,
    
    -- 关联订单（锁定/使用时记录）
    order_id BIGINT UNSIGNED DEFAULT NULL COMMENT '关联订单ID',
    
    -- 有效期（从模板计算得出）
    valid_from DATETIME NOT NULL COMMENT '生效时间',
    valid_to DATETIME NOT NULL COMMENT '失效时间',
    
    -- 使用时间
    used_at DATETIME DEFAULT NULL COMMENT '使用时间',
    
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    INDEX idx_user_id_status (user_id, status),
    INDEX idx_user_template (user_id, template_id),
    INDEX idx_order_id (order_id),
    INDEX idx_valid_to (valid_to)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户优惠券表';


-- 3. 优惠券操作记录表（幂等）
CREATE TABLE coupon_operations (
    id BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    
    operation_id VARCHAR(64) NOT NULL COMMENT '操作唯一ID（幂等键）',
    user_coupon_id BIGINT UNSIGNED NOT NULL COMMENT '用户优惠券ID',
    order_id BIGINT UNSIGNED DEFAULT NULL COMMENT '订单ID',
    
    -- 操作类型: ISSUE/RESERVE/COMMIT/RELEASE/REFUND
    operation_type VARCHAR(16) NOT NULL,
    
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE KEY uk_operation_id (operation_id),
    INDEX idx_user_coupon_id (user_coupon_id),
    INDEX idx_order_id (order_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='优惠券操作记录表';


-- ====================
-- 订单表需要增加的字段（供你参考）
-- ====================
-- ALTER TABLE orders ADD COLUMN coupon_id BIGINT UNSIGNED DEFAULT NULL COMMENT '使用的优惠券ID';
-- ALTER TABLE orders ADD COLUMN coupon_discount INT NOT NULL DEFAULT 0 COMMENT '优惠券折扣金额（分）';
-- 
-- 订单金额计算逻辑：
-- actual_amount = product_amount + shipping_fee - coupon_discount
-- 
-- 其中：
-- product_amount: 商品总金额
-- shipping_fee: 运费
-- coupon_discount: 优惠券折扣金额
