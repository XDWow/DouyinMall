if redis.call("GET", KEYS[3]) == ARGV[3] then
    return 0
end

local marker = redis.call("GET", KEYS[2])
if marker == ARGV[2] then
    redis.call("INCRBY", KEYS[1], ARGV[1])
    redis.call("DEL", KEYS[2])
end

redis.call("SET", KEYS[3], ARGV[3], "EX", ARGV[5])
redis.call("SET", KEYS[4], ARGV[4], "EX", ARGV[5])
return 1
