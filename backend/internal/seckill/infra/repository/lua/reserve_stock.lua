if redis.call("EXISTS", KEYS[2]) == 1 then
    return 2
end

local stock = tonumber(redis.call("GET", KEYS[1]))
if not stock or stock <= 0 then
    return 1
end

redis.call("DECRBY", KEYS[1], ARGV[1])
redis.call("SET", KEYS[2], ARGV[3], "EX", ARGV[2])
return 0
