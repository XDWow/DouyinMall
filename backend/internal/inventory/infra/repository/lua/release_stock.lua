-- ReleaseStock: 订单取消，恢复Redis预库存
-- KEYS[1] = release:{releaseID} (本次释放操作的幂等key)
-- KEYS[2] = reserve:{reserveID} (预扣记录，只读不删)
-- ARGV[1] = expireTime (幂等key过期时间，秒)
-- 返回：1=成功释放, 0=已释放过(幂等), -1=预扣记录不存在

local releaseKey = KEYS[1]
local reserveKey = KEYS[2]
local expireTime = tonumber(ARGV[1])

-- 1. 幂等检查：如果释放记录已存在，说明已释放过
if redis.call('EXISTS', releaseKey) == 1 then
    return 0  -- 已释放过，幂等返回
end

-- 2. 检查预扣记录是否存在
if redis.call('EXISTS', reserveKey) == 0 then
    return -1  -- 预扣记录不存在（可能已过期或从未预扣）
end

-- 3. 读取预扣记录的所有商品数据
local data = redis.call('HGETALL', reserveKey)

-- 4. 恢复库存（记录中quantity是负数，需要取绝对值）
for i = 1, #data, 2 do
    local productID = data[i]
    local quantity = tonumber(data[i + 1])
    if quantity then
        -- quantity是负数（如-5），取绝对值后INCRBY增加库存
        redis.call('INCRBY', 'stock:' .. productID, -quantity)
    end
end

-- 5. 设置释放幂等key（不删除预扣记录，让其自然过期）
redis.call('SETEX', releaseKey, expireTime, '1')

return 1  -- 释放成功
