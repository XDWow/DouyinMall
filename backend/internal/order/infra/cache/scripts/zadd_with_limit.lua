-- ZAddWithLimit: 原子性执行 ZADD + 裁剪 + TTL
-- KEYS[1]: zset key
-- ARGV[1]: limit (保留的最大元素数量)
-- ARGV[2]: ttl (过期时间，秒)
-- ARGV[3..n]: score1, member1, score2, member2, ...

local key = KEYS[1]
local limit = tonumber(ARGV[1])
local ttl = tonumber(ARGV[2])

-- ZADD，从 ARGV[3] 开始是 score member 对
local members = {}
for i = 3, #ARGV, 2 do
    table.insert(members, ARGV[i])     -- score
    table.insert(members, ARGV[i+1])   -- member
end
redis.call('ZADD', key, unpack(members))

-- 裁剪，只保留top limit个（按score降序）
local count = redis.call('ZCARD', key)
if count > limit then
    redis.call('ZREMRANGEBYRANK', key, 0, count - limit - 1)
end

-- 设置TTL
if ttl > 0 then
    redis.call('EXPIRE', key, ttl)
end

return redis.status_reply('OK')
