package usecase

import (
	"context"
	"fmt"

	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
	"github.com/XDWow/DouyinMall/backend/pkg/ai"
	"github.com/XDWow/DouyinMall/backend/pkg/logger"
)

// SyncUseCase 鏁版嵁鍚屾锛圞afka 浜嬩欢 鈫?ES锛?
type SyncUseCase struct {
	productRepo  domain.ProductRepo
	merchantRepo domain.MerchantRepo
	embedder     ai.Embedder
	l            logger.LoggerV1
}

func NewSyncUseCase(productRepo domain.ProductRepo, merchantRepo domain.MerchantRepo, embedder ai.Embedder, l logger.LoggerV1) *SyncUseCase {
	return &SyncUseCase{productRepo: productRepo, merchantRepo: merchantRepo, embedder: embedder, l: l}
}

// Sync 澶勭悊鍗曚釜鍚屾浜嬩欢
func (uc *SyncUseCase) Sync(ctx context.Context, event domain.SyncEvent) error {
	switch event.Type {
	case domain.EventTypeProduct:
		// 鍐欏叆鍓嶇敓鎴愬悜閲?
		if event.Action != domain.EventActionDelete && event.Product != nil {
			uc.enrichWithVector(ctx, event.Product)
		}
		return uc.productRepo.SyncProduct(ctx, string(event.Action), event.Product)
	case domain.EventTypeMerchant:
		return uc.merchantRepo.SyncMerchant(ctx, string(event.Action), event.Merchant)
	default:
		return fmt.Errorf("鏈煡浜嬩欢绫诲瀷: %s", event.Type)
	}
}

// BatchSync 鎵归噺鍚屾
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

// enrichWithVector 涓哄晢鍝佹枃妗ｇ敓鎴愬悕绉板悜閲?
func (uc *SyncUseCase) enrichWithVector(ctx context.Context, doc *domain.ProductDocument) {
	if doc.Name == "" {
		return
	}
	vectors, err := uc.embedder.Embed(ctx, []string{doc.Name})
	if err != nil {
		uc.l.Warn("鐢熸垚鍟嗗搧鍚戦噺澶辫触", logger.Error(err), logger.Int64("product_id", doc.ID))
		return
	}
	if len(vectors) > 0 {
		doc.NameVector = vectors[0]
	}
}


