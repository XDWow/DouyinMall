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

//go:embed lua/compensate.lua
var compensateLua string

//go:embed lua/resolve_transaction.lua
var resolveTransactionLua string

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
func resultDataKey(requestNo string) string   { return fmt.Sprintf("seckill:req:data:%s", requestNo) }
func resultStatusKey(requestNo string) string { return fmt.Sprintf("seckill:req:status:%s", requestNo) }

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
	result := domain.Result{
		RequestNo: requestNo,
		Status:    domain.RequestStatusProcessing,
	}
	resultData, err := json.Marshal(result)
	if err != nil {
		return 0, err
	}
	return r.cache.EvalInt64(
		ctx,
		reserveStockLua,
		[]string{
			stockKey(activityID),
			userKey(activityID, userID),
			resultStatusKey(requestNo),
			resultDataKey(requestNo),
		},
		1,
		userTTLSeconds,
		requestNo,
		result.Status,
		string(resultData),
		int64(resultTTL.Seconds()),
	)
}

func (r *CacheRepository) Compensate(ctx context.Context, activityID, userID int64, requestNo string, result domain.Result) error {
	resultData, err := json.Marshal(result)
	if err != nil {
		return err
	}
	_, err = r.cache.EvalInt64(
		ctx,
		compensateLua,
		[]string{
			stockKey(activityID),
			userKey(activityID, userID),
			resultStatusKey(requestNo),
			resultDataKey(requestNo),
		},
		1,
		requestNo,
		result.Status,
		string(resultData),
		int64(resultTTL.Seconds()),
	)
	return err
}

func (r *CacheRepository) SetResult(ctx context.Context, result domain.Result) error {
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	pipe, err := r.cache.Pipeline()
	if err != nil {
		return err
	}
	pipe.Set(ctx, resultStatusKey(result.RequestNo), result.Status, resultTTL)
	pipe.Set(ctx, resultDataKey(result.RequestNo), string(data), resultTTL)
	_, err = pipe.Exec(ctx)
	return err
}

func (r *CacheRepository) GetResult(ctx context.Context, requestNo string) (*domain.Result, error) {
	val, err := r.cache.Get(ctx, resultDataKey(requestNo))
	if err == nil {
		var result domain.Result
		if err = json.Unmarshal([]byte(val), &result); err != nil {
			return nil, err
		}
		return &result, nil
	}
	if err != redis.Nil {
		return nil, err
	}

	status, statusErr := r.cache.Get(ctx, resultStatusKey(requestNo))
	if statusErr == redis.Nil {
		return nil, nil
	}
	if statusErr != nil {
		return nil, statusErr
	}
	return &domain.Result{
		RequestNo: requestNo,
		Status:    status,
	}, nil
}

func (r *CacheRepository) ResolveTransaction(ctx context.Context, activityID, userID int64, requestNo string) (domain.TransactionResolution, error) {
	result := domain.Result{
		RequestNo: requestNo,
		Status:    domain.RequestStatusProcessing,
	}
	resultData, err := json.Marshal(result)
	if err != nil {
		return domain.TransactionResolutionUnknown, err
	}
	code, err := r.cache.EvalInt64(
		ctx,
		resolveTransactionLua,
		[]string{
			userKey(activityID, userID),
			resultStatusKey(requestNo),
			resultDataKey(requestNo),
		},
		requestNo,
		result.Status,
		string(resultData),
		int64(resultTTL.Seconds()),
	)
	if err != nil {
		return domain.TransactionResolutionUnknown, err
	}

	switch code {
	case 1:
		return domain.TransactionResolutionCommit, nil
	case 2:
		return domain.TransactionResolutionRollback, nil
	default:
		return domain.TransactionResolutionUnknown, nil
	}
}
