local status = redis.call("GET", KEYS[2])
if status then
    if status == "FAILED" then
        return 2
    end
    return 1
end

local marker = redis.call("GET", KEYS[1])
if marker == ARGV[1] then
    redis.call("SET", KEYS[2], ARGV[2], "EX", ARGV[4])
    redis.call("SET", KEYS[3], ARGV[3], "EX", ARGV[4])
    return 1
end

if not marker then
    return 1
end

return 2
