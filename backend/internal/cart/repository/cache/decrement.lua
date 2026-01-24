-- 减少购物车商品数量（quantity > 1 才能减）
-- KEYS[1]: cart key
-- ARGV[1]: product_id (field)
-- 返回：新数量 或 -1（不能减）

local qty = redis.call('HGET', KEYS[1], ARGV[1])
if not qty then
    return -1
end

qty = tonumber(qty)
if qty <= 1 then
    return -1
end

local newQty = redis.call('HINCRBY', KEYS[1], ARGV[1], -1)
redis.call('EXPIRE', KEYS[1], ARGV[2])
return newQty

