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

	return &v1.ListProductsResp{
		Products: toProtoList(products),
	}, nil
}

func (h *ProductHandler) GetProducts(ctx context.Context, req *v1.GetProductsReq) (*v1.GetProductsResp, error) {
	ids := req.GetId()
	products := make([]*v1.Product, 0, len(ids))
	for _, id := range ids {
		product, err := h.svc.GetProduct(ctx, id)
		if err != nil {
			return nil, err
		}
		products = append(products, toProto(product))
	}

	return &v1.GetProductsResp{
		Product: products,
	}, nil
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
	err := h.svc.DeleteProduct(ctx, req.GetId(), req.GetUserId())
	if err != nil {
		return nil, err
	}

	return &v1.DeleteProductResp{}, nil
}

func toProto(p domain.Product) *v1.Product {
	return &v1.Product{
		Id:           p.ID,
		Name:         p.Name,
		Description:  p.Description,
		Picture:      p.Picture,
		SliderImgs:   p.SlideImgs,
		Price:        p.Price,
		Categories:   p.Categories,
		InStock:      p.InStock, // 直接使用 in_stock（来自库存服务）
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
	return domain.Product{
		ID:           p.GetId(),
		Name:         p.GetName(),
		Description:  p.GetDescription(),
		Picture:      p.GetPicture(),
		SlideImgs:    p.GetSliderImgs(),
		Price:        p.GetPrice(),
		Categories:   p.GetCategories(),
		InStock:      p.GetInStock(), // 直接使用 in_stock（库存服务管理）
		MerchantID:   p.GetMerchantID(),
		MerchantName: p.GetMerchantName(),
	}
}
