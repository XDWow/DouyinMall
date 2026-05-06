package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/XDWow/DouyinMall/backend/internal/seckill/domain"
	"github.com/stretchr/testify/require"
)

func TestSubmitUseCaseCommitOnReserveSuccess(t *testing.T) {
	cache := &submitCacheStub{reserveCode: 0}
	tx := &submitTxStub{}
	producer := &submitProducerStub{tx: tx}
	uc := NewSubmitUseCase(&submitActivityRepoStub{}, nil, cache, producer, submitIDGenStub("1001"))

	result, err := uc.Execute(context.Background(), SubmitCmd{
		ActivityID: 1,
		UserID:     9,
	})

	require.NoError(t, err)
	require.Equal(t, domain.RequestStatusProcessing, result.Status)
	require.True(t, tx.committed)
	require.False(t, tx.rolledBack)
	require.Equal(t, "1001", producer.lastEvent.RequestNo)
}

func TestSubmitUseCaseRollbackOnDuplicate(t *testing.T) {
	cache := &submitCacheStub{reserveCode: 2}
	tx := &submitTxStub{}
	producer := &submitProducerStub{tx: tx}
	uc := NewSubmitUseCase(&submitActivityRepoStub{}, nil, cache, producer, submitIDGenStub("1002"))

	result, err := uc.Execute(context.Background(), SubmitCmd{
		ActivityID: 1,
		UserID:     9,
	})

	require.ErrorIs(t, err, domain.ErrDuplicateSeckill)
	require.Equal(t, domain.RequestStatusFailed, result.Status)
	require.True(t, tx.rolledBack)
	require.False(t, tx.committed)
}

func TestSubmitUseCaseLeavesHalfMessageOnReserveError(t *testing.T) {
	cache := &submitCacheStub{reserveErr: errors.New("redis timeout")}
	tx := &submitTxStub{}
	producer := &submitProducerStub{tx: tx}
	uc := NewSubmitUseCase(&submitActivityRepoStub{}, nil, cache, producer, submitIDGenStub("1003"))

	result, err := uc.Execute(context.Background(), SubmitCmd{
		ActivityID: 1,
		UserID:     9,
	})

	require.NoError(t, err)
	require.Equal(t, domain.RequestStatusProcessing, result.Status)
	require.False(t, tx.committed)
	require.False(t, tx.rolledBack)
}

type submitActivityRepoStub struct{}

func (s *submitActivityRepoStub) Create(context.Context, *domain.Activity) error { return nil }

func (s *submitActivityRepoStub) FindByID(context.Context, int64) (*domain.Activity, error) {
	return &domain.Activity{
		ID:           1,
		ProductID:    11,
		SKUID:        12,
		SeckillPrice: 99,
		Status:       domain.ActivityStatusOnline,
		StartTime:    time.Now().Add(-time.Minute),
		EndTime:      time.Now().Add(time.Minute),
	}, nil
}

func (s *submitActivityRepoStub) UpdateStatus(context.Context, int64, string) error { return nil }

type submitCacheStub struct {
	reserveCode int64
	reserveErr  error
}

func (s *submitCacheStub) SetActivity(context.Context, *domain.Activity) error { return nil }

func (s *submitCacheStub) GetActivity(context.Context, int64) (*domain.Activity, error) {
	return nil, nil
}

func (s *submitCacheStub) SetActivityStock(context.Context, int64, int32) error { return nil }

func (s *submitCacheStub) AtomicReserve(context.Context, int64, int64, string, int64) (int64, error) {
	return s.reserveCode, s.reserveErr
}

func (s *submitCacheStub) Compensate(context.Context, int64, int64, string, domain.Result) error {
	return nil
}

func (s *submitCacheStub) SetResult(context.Context, domain.Result) error { return nil }

func (s *submitCacheStub) GetResult(context.Context, string) (*domain.Result, error) { return nil, nil }

func (s *submitCacheStub) ResolveTransaction(context.Context, int64, int64, string) (domain.TransactionResolution, error) {
	return domain.TransactionResolutionUnknown, nil
}

type submitProducerStub struct {
	tx        *submitTxStub
	lastEvent domain.Event
}

func (s *submitProducerStub) Prepare(_ context.Context, evt domain.Event) (domain.Transaction, error) {
	s.lastEvent = evt
	return s.tx, nil
}

type submitTxStub struct {
	committed  bool
	rolledBack bool
}

func (s *submitTxStub) Commit() error {
	s.committed = true
	return nil
}

func (s *submitTxStub) Rollback() error {
	s.rolledBack = true
	return nil
}

type submitIDGenStub string

func (s submitIDGenStub) GenerateID() string { return string(s) }
