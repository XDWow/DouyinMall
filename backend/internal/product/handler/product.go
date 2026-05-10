package handler

import (
	"context"

	"github.com/XDWow/DouyinMall/backend/internal/product/domain"
	"github.com/XDWow/DouyinMall/backend/internal/product/service"
	v1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/product/v1"
)

type ProductHandler struct {
	svc service.ProductService
}

func NewProductHandler(svc service.ProductService) *ProductHandler {
	return &ProductHandler{svc: svc}
}

func (h *ProductHandler) ListProducts(ctx context.Context, req *v1.ListProductsReq) (*v1.ListProductsResp, error) {
	products, err := h.svc.ListProducts(ctx, req.GetPage(), req.GetPageSize(), req.GetCategory())
	if err != nil {
		return nil, err
	}
	return &v1.ListProductsResp{Products: toProtoList(products)}, nil
}

func (h *ProductHandler) GetProducts(ctx context.Context, req *v1.GetProductsReq) (*v1.GetProductsResp, error) {
	items := req.GetItems()
	products := make([]*v1.Product, 0, len(items))
	for _, item := range items {
		product, err := h.svc.GetProduct(ctx, domain.ProductQuery{
			ID:    item.GetProductId(),
			SKUID: item.GetSkuId(),
		})
		if err != nil {
			return nil, err
		}
		products = append(products, toProto(product))
	}
	return &v1.GetProductsResp{Product: products}, nil
}

func (h *ProductHandler) CreateProduct(ctx context.Context, req *v1.CreateProductReq) (*v1.CreateProductResp, error) {
	id, err := h.svc.CreateProduct(ctx, toDomain(req.GetProduct()))
	if err != nil {
		return nil, err
	}
	return &v1.CreateProductResp{Id: id}, nil
}

func (h *ProductHandler) UpdateProduct(ctx context.Context, req *v1.UpdateProductReq) (*v1.UpdateProductResp, error) {
	id, err := h.svc.UpdateProduct(ctx, toDomain(req.GetProduct()))
	if err != nil {
		return nil, err
	}
	return &v1.UpdateProductResp{Id: id}, nil
}

func (h *ProductHandler) DeleteProduct(ctx context.Context, req *v1.DeleteProductReq) (*v1.DeleteProductResp, error) {
	if err := h.svc.DeleteProduct(ctx, req.GetId(), req.GetUserId()); err != nil {
		return nil, err
	}
	return &v1.DeleteProductResp{}, nil
}

func toProto(p domain.Product) *v1.Product {
	return &v1.Product{
		Id:           p.ID,
		SkuId:        p.SKUID,
		Name:         p.Name,
		Description:  p.Description,
		Picture:      p.Picture,
		SliderImgs:   p.SlideImgs,
		Price:        p.Price,
		Currency:     p.Currency,
		Categories:   p.Categories,
		InStock:      p.InStock,
		MerchantID:   p.MerchantID,
		MerchantName: p.MerchantName,
	}
}

func toProtoList(products []domain.Product) []*v1.Product {
	res := make([]*v1.Product, len(products))
	for i, p := range products {
		res[i] = toProto(p)
	}
	return res
}

func toDomain(p *v1.Product) domain.Product {
	if p == nil {
		return domain.Product{}
	}
	return domain.Product{
		ID:           p.GetId(),
		SKUID:        p.GetSkuId(),
		Name:         p.GetName(),
		Description:  p.GetDescription(),
		Picture:      p.GetPicture(),
		SlideImgs:    p.GetSliderImgs(),
		Price:        p.GetPrice(),
		Currency:     p.GetCurrency(),
		Categories:   p.GetCategories(),
		InStock:      p.GetInStock(),
		MerchantID:   p.GetMerchantID(),
		MerchantName: p.GetMerchantName(),
	}
}
