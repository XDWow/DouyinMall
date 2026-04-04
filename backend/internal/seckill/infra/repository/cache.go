package repository

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	rediscache "github.com/XDWow/DouyinMall/backend/internal/seckill/infra/cache"
	"github.com/redis/go-redis/v9"
)

const (
	activityTTL = time.Hour
	resultTTL   = 24 * time.Hour
)

//go:embed lua/reserve_stock.lua
var reserveStockLua string

type CacheRepository struct {
	cache *rediscache.RedisCache
}

func NewCacheRepository(cache *rediscache.RedisCache) domain.Cache {
	return &CacheRepository{cache: cache}
}

func activityKey(activityID int64) string { return fmt.Sprintf("seckill:activity:%d", activityID) }
func stockKey(activityID int64) string    { return fmt.Sprintf("seckill:stock:%d", activityID) }
func userKey(activityID, userID int64) string {
	return fmt.Sprintf("seckill:user:%d:%d", activityID, userID)
}
func resultKey(requestNo string) string { return fmt.Sprintf("seckill:req:%s", requestNo) }

func (r *CacheRepository) SetActivity(ctx context.Context, activity *domain.Activity) error {
	data, err := json.Marshal(activity)
	if err != nil {
		return err
	}
	return r.cache.Set(ctx, activityKey(activity.ID), data, activityTTL)
}

func (r *CacheRepository) GetActivity(ctx context.Context, activityID int64) (*domain.Activity, error) {
	val, err := r.cache.Get(ctx, activityKey(activityID))
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var activity domain.Activity
	if err := json.Unmarshal([]byte(val), &activity); err != nil {
		return nil, err
	}
	return &activity, nil
}

func (r *CacheRepository) SetActivityStock(ctx context.Context, activityID int64, stock int32) error {
	return r.cache.Set(ctx, stockKey(activityID), stock, 0)
}

func (r *CacheRepository) AtomicReserve(ctx context.Context, activityID, userID int64, requestNo string, userTTLSeconds int64) (int64, error) {
	resultData, err := json.Marshal(domain.Result{
		RequestNo: requestNo,
		Status:    domain.RequestStatusProcessing,
	})
	if err != nil {
		return 0, err
	}
	return r.cache.EvalInt64(
		ctx,
		reserveStockLua,
		[]string{stockKey(activityID), userKey(activityID, userID), resultKey(requestNo)},
		1,
		userTTLSeconds,
		string(resultData),
		int64(resultTTL.Seconds()),
	)
}

func (r *CacheRepository) Compensate(ctx context.Context, activityID, userID int64, quantity int32, removeUser bool) error {
	pipe, err := r.cache.Pipeline()
	if err != nil {
		return err
	}
	pipe.IncrBy(ctx, stockKey(activityID), int64(quantity))
	if removeUser {
		pipe.Del(ctx, userKey(activityID, userID))
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (r *CacheRepository) IncreaseStock(ctx context.Context, activityID int64, quantity int32) error {
	return r.cache.IncrBy(ctx, stockKey(activityID), int64(quantity))
}

func (r *CacheRepository) SetResult(ctx context.Context, result domain.Result) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return r.cache.Set(ctx, resultKey(result.RequestNo), data, resultTTL)
}

func (r *CacheRepository) GetResult(ctx context.Context, requestNo string) (*domain.Result, error) {
	val, err := r.cache.Get(ctx, resultKey(requestNo))
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var result domain.Result
	if err := json.Unmarshal([]byte(val), &result); err != nil {
		return nil, err
	}
	return &result, nil
}


