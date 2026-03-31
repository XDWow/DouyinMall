-- ZAddWithLimit: 原子执行 ZADD + 裁剪 + TTL
-- KEYS[1]: zset key
-- ARGV[1]: limit
-- ARGV[2]: ttl seconds
-- ARGV[3..n]: score1, member1, score2, member2, ...

local key = KEYS[1]
local limit = tonumber(ARGV[1])
local ttl = tonumber(ARGV[2])

local members = {}
for i = 3, #ARGV, 2 do
    table.insert(members, ARGV[i])
    table.insert(members, ARGV[i + 1])
end
redis.call('ZADD', key, unpack(members))

local count = redis.call('ZCARD', key)
if count > limit then
    redis.call('ZREMRANGEBYRANK', key, 0, count - limit - 1)
end

if ttl > 0 then
    redis.call('EXPIRE', key, ttl)
end

return redis.status_reply('OK')
