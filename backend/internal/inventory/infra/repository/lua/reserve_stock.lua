
local reserveKey = KEYS[1]
local expireTime = tonumber(ARGV[1])

-- 1. 幂等检查：如果预扣记录已存在，直接返回成功
if redis.call('EXISTS', reserveKey) == 1 then
    return 0  -- 已存在，幂等返回成功
end

-- 2. 检查所有商品库存是否充足，收集不足明细
local productCount = (#ARGV - 1) / 2
local insufficientItems = {}

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
    local availableStock = tonumber(stock)
    if availableStock < requiredStock then
        -- 记录：productID, 请求数量, 可用库存
        table.insert(insufficientItems, productID)
        table.insert(insufficientItems, requiredStock)
        table.insert(insufficientItems, availableStock)
    end
end

-- 有任意商品库存不足，整体失败，返回所有不足明细
if #insufficientItems > 0 then
    return insufficientItems
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
