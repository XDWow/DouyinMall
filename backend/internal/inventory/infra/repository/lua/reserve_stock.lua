-- 预扣库存 Lua 脚本（原子性操作）
-- KEYS[1] = reserve:{reserveID}
-- ARGV[1] = expireTime (秒，订单30分钟超时 + 5分钟缓冲 = 35分钟 = 2100秒)
-- ARGV[2..n] = productID1, quantity1, productID2, quantity2, ...
-- 
-- 为什么要比订单超时长？
-- 避免边界case：订单刚好30分钟时取消，但Redis记录已过期，导致ReleaseStock找不到记录
-- 
-- 为什么不删除预扣记录？
-- ReleaseStock需要从预扣记录读取商品信息，且预扣记录用于预扣幂等（防止重复预扣）

local reserveKey = KEYS[1]
local expireTime = tonumber(ARGV[1])

-- 1. 幂等检查：如果预扣记录已存在，直接返回成功
if redis.call('EXISTS', reserveKey) == 1 then
    return 0  -- 已存在，幂等返回成功
end

-- 2. 检查所有商品库存是否充足
local productCount = (#ARGV - 1) / 2
for i = 1, productCount do
    local productID = ARGV[i * 2]
    local quantity = tonumber(ARGV[i * 2 + 1])  -- 带符号（负数表示扣减）
    local stockKey = 'stock:' .. productID
    
    local stock = redis.call('GET', stockKey)
    if not stock then
        return -1  -- 商品不存在
    end
    
    -- quantity是负数（如-5），需要扣减的数量是其绝对值
    local requiredStock = -quantity
    if tonumber(stock) < requiredStock then
        return -2  -- 库存不足
    end
end

-- 3. 扣减库存 + 保存预扣记录
for i = 1, productCount do
    local productID = ARGV[i * 2]
    local quantity = tonumber(ARGV[i * 2 + 1])  -- 带符号的数量（负数表示扣减）
    local stockKey = 'stock:' .. productID
    
    -- 扣减库存（统一用INCRBY，quantity是负数时自动减少）
    redis.call('INCRBY', stockKey, quantity)
    
    -- 保存预扣记录（保存原始带符号的值，-5表示扣减了5个）
    redis.call('HSET', reserveKey, productID, quantity)
end

-- 4. 设置过期时间
redis.call('EXPIRE', reserveKey, expireTime)

return 1  -- 成功
