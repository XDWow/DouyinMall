local qty = redis.call('HGET', KEYS[1], ARGV[1])
if not qty then
    return -1
end

qty = tonumber(qty)
if qty <= 1 then
    return -1
end

local new_qty = redis.call('HINCRBY', KEYS[1], ARGV[1], -1)
redis.call('EXPIRE', KEYS[1], ARGV[2])
return new_qty
