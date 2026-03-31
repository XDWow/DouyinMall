package usecase

import (
	"context"
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/ai"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// SyncUseCase 数据同步（Kafka 事件 → ES）
type SyncUseCase struct {
	productRepo  domain.ProductRepo
	merchantRepo domain.MerchantRepo
	embedder     ai.Embedder
	l            logger.LoggerV1
}

func NewSyncUseCase(productRepo domain.ProductRepo, merchantRepo domain.MerchantRepo, embedder ai.Embedder, l logger.LoggerV1) *SyncUseCase {
	return &SyncUseCase{productRepo: productRepo, merchantRepo: merchantRepo, embedder: embedder, l: l}
}

// Sync 处理单个同步事件
func (uc *SyncUseCase) Sync(ctx context.Context, event domain.SyncEvent) error {
	switch event.Type {
	case domain.EventTypeProduct:
		// 写入前生成向量
		if event.Action != domain.EventActionDelete && event.Product != nil {
			uc.enrichWithVector(ctx, event.Product)
		}
		return uc.productRepo.SyncProduct(ctx, string(event.Action), event.Product)
	case domain.EventTypeMerchant:
		return uc.merchantRepo.SyncMerchant(ctx, string(event.Action), event.Merchant)
	default:
		return fmt.Errorf("未知事件类型: %s", event.Type)
	}
}

// BatchSync 批量同步
func (uc *SyncUseCase) BatchSync(ctx context.Context, events []domain.SyncEvent) (success, failed int64, errors []string) {
	var productDocs []domain.ProductDocument
	var merchantDocs []domain.MerchantDocument
	var productDelIDs, merchantDelIDs []int64

	for _, e := range events {
		switch e.Type {
		case domain.EventTypeProduct:
			if e.Action == domain.EventActionDelete {
				productDelIDs = append(productDelIDs, e.ID)
			} else if e.Product != nil {
				uc.enrichWithVector(ctx, e.Product)
				productDocs = append(productDocs, *e.Product)
			}
		case domain.EventTypeMerchant:
			if e.Action == domain.EventActionDelete {
				merchantDelIDs = append(merchantDelIDs, e.ID)
			} else if e.Merchant != nil {
				merchantDocs = append(merchantDocs, *e.Merchant)
			}
		}
	}

	if len(productDocs) > 0 {
		s, f, errs := uc.productRepo.BatchSyncProducts(ctx, productDocs)
		success += s
		failed += f
		errors = append(errors, errs...)
	}
	if len(productDelIDs) > 0 {
		s, f, errs := uc.productRepo.BatchDeleteProducts(ctx, productDelIDs)
		success += s
		failed += f
		errors = append(errors, errs...)
	}
	if len(merchantDocs) > 0 {
		s, f, errs := uc.merchantRepo.BatchSyncMerchants(ctx, merchantDocs)
		success += s
		failed += f
		errors = append(errors, errs...)
	}
	if len(merchantDelIDs) > 0 {
		s, f, errs := uc.merchantRepo.BatchDeleteMerchants(ctx, merchantDelIDs)
		success += s
		failed += f
		errors = append(errors, errs...)
	}
	return
}

// enrichWithVector 为商品文档生成名称向量
func (uc *SyncUseCase) enrichWithVector(ctx context.Context, doc *domain.ProductDocument) {
	if doc.Name == "" {
		return
	}
	vectors, err := uc.embedder.Embed(ctx, []string{doc.Name})
	if err != nil {
		uc.l.Warn("生成商品向量失败", logger.Error(err), logger.Int64("product_id", doc.ID))
		return
	}
	if len(vectors) > 0 {
		doc.NameVector = vectors[0]
	}
}
