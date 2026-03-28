-- ZClaimByScore: 原子领取 score <= max 的一批 member
-- KEYS[1]: zset key
-- ARGV[1]: max score
-- ARGV[2]: limit

local key = KEYS[1]
local max_score = ARGV[1]
local limit = tonumber(ARGV[2])

local members
if limit ~= nil and limit > 0 then
    members = redis.call('ZRANGEBYSCORE', key, '-inf', max_score, 'LIMIT', 0, limit)
else
    members = redis.call('ZRANGEBYSCORE', key, '-inf', max_score)
end

if #members == 0 then
    return {}
end

redis.call('ZREM', key, unpack(members))
return members
