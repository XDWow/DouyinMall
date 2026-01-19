package service

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
	"github.com/XDWow/DouyinMall/backend/internal/search/repo"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

type SyncService interface {
	Sync(ctx context.Context, event domain.SyncEvent) error
	BatchSync(ctx context.Context, events []domain.SyncEvent) (successCount, failedCount int64, errors []string)
}

type syncService struct {
	productRepo  repo.ProductRepo
	merchantRepo repo.MerchantRepo
	logger       logger.LoggerV1
}

func NewSyncService(productRepo repo.ProductRepo, merchantRepo repo.MerchantRepo, logger logger.LoggerV1) SyncService {
	return &syncService{
		productRepo:  productRepo,
		merchantRepo: merchantRepo,
		logger:       logger,
	}
}

func (s *syncService) Sync(ctx context.Context, event domain.SyncEvent) error {
	switch event.Type {
	case domain.EventTypeProduct:
		return s.syncProduct(ctx, event)
	case domain.EventTypeMerchant:
		return s.syncMerchant(ctx, event)
	default:
		return nil
	}
}

func (s *syncService) BatchSync(ctx context.Context, events []domain.SyncEvent) (successCount, failedCount int64, errors []string) {
	if len(events) == 0 {
		return 0, 0, nil
	}

	// 根据第一个事件的类型决定批量处理方式（同一批次不会混合类型）
	switch events[0].Type {
	case domain.EventTypeProduct:
		return s.batchSyncProducts(ctx, events)
	case domain.EventTypeMerchant:
		return s.batchSyncMerchants(ctx, events)
	default:
		return 0, int64(len(events)), []string{"未知的事件类型"}
	}
}

func (s *syncService) syncProduct(ctx context.Context, event domain.SyncEvent) error {
	switch event.Action {
	case domain.EventActionDelete:
		return s.productRepo.DeleteProduct(ctx, event.ID)
	case domain.EventActionCreate, domain.EventActionUpdate:
		if event.Product == nil {
			return nil
		}
		action := "CREATE"
		if event.Action == domain.EventActionUpdate {
			action = "UPDATE"
		}
		return s.productRepo.SyncProduct(ctx, action, event.Product)
	default:
		return nil
	}
}

func (s *syncService) syncMerchant(ctx context.Context, event domain.SyncEvent) error {
	switch event.Action {
	case domain.EventActionDelete:
		return s.merchantRepo.DeleteMerchant(ctx, event.ID)
	case domain.EventActionCreate, domain.EventActionUpdate:
		if event.Merchant == nil {
			return nil
		}
		action := "CREATE"
		if event.Action == domain.EventActionUpdate {
			action = "UPDATE"
		}
		return s.merchantRepo.SyncMerchant(ctx, action, event.Merchant)
	default:
		return nil
	}
}

func (s *syncService) batchSyncProducts(ctx context.Context, events []domain.SyncEvent) (successCount, failedCount int64, errors []string) {
	var productDocs []domain.ProductDocument
	var deleteIDs []int64
	var errList []string

	// 分类收集：DELETE 和 CREATE/UPDATE
	for _, event := range events {
		if event.Action == domain.EventActionDelete {
			if event.ID == 0 {
				failedCount++
				errList = append(errList, "删除商品事件缺少 ID")
				continue
			}
			deleteIDs = append(deleteIDs, event.ID)
		} else {
			// CREATE/UPDATE 需要数据
			if event.Product == nil {
				failedCount++
				errList = append(errList, "商品事件缺少数据")
				continue
			}
			productDocs = append(productDocs, *event.Product)
		}
	}

	// 批量删除
	if len(deleteIDs) > 0 {
		success, failed, errs := s.productRepo.BatchDeleteProducts(ctx, deleteIDs)
		successCount += success
		failedCount += failed
		errList = append(errList, errs...)
	}

	// 批量同步（CREATE/UPDATE）
	if len(productDocs) > 0 {
		success, failed, errs := s.productRepo.BatchSyncProducts(ctx, productDocs)
		successCount += success
		failedCount += failed
		errList = append(errList, errs...)
	}

	return successCount, failedCount, errList
}

func (s *syncService) batchSyncMerchants(ctx context.Context, events []domain.SyncEvent) (successCount, failedCount int64, errors []string) {
	var merchantDocs []domain.MerchantDocument
	var deleteIDs []int64
	var errList []string

	// 分类收集：DELETE 和 CREATE/UPDATE
	for _, event := range events {
		if event.Action == domain.EventActionDelete {
			if event.ID == 0 {
				failedCount++
				errList = append(errList, "删除商家事件缺少 ID")
				continue
			}
			deleteIDs = append(deleteIDs, event.ID)
		} else {
			// CREATE/UPDATE 需要数据
			if event.Merchant == nil {
				failedCount++
				errList = append(errList, "商家事件缺少数据")
				continue
			}
			merchantDocs = append(merchantDocs, *event.Merchant)
		}
	}

	// 批量删除
	if len(deleteIDs) > 0 {
		success, failed, errs := s.merchantRepo.BatchDeleteMerchants(ctx, deleteIDs)
		successCount += success
		failedCount += failed
		errList = append(errList, errs...)
	}

	// 批量同步（CREATE/UPDATE）
	if len(merchantDocs) > 0 {
		success, failed, errs := s.merchantRepo.BatchSyncMerchants(ctx, merchantDocs)
		successCount += success
		failedCount += failed
		errList = append(errList, errs...)
	}

	return successCount, failedCount, errList
}
