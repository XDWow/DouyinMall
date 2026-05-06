package grpc

import (
	"github.com/XDWow/DouyinMall/backend/internal/search/domain"
	searchv1 "github.com/XDWow/DouyinMall/backend/rpc_gen/kitex_gen/search/v1"
)

// ==================== Proto 鈫?Domain ====================

func toDomainSearchProductsReq(req *searchv1.SearchProductsReq) *domain.SearchProductsReq {
	return &domain.SearchProductsReq{
		Keyword: req.GetKeyword(), Page: req.GetPage(), PageSize: req.GetPageSize(),
		Categories: req.GetCategories(), MinPrice: req.GetMinPrice(), MaxPrice: req.GetMaxPrice(),
		MerchantID: req.GetMerchantId(), InStockOnly: true,
		SortBy: req.GetSortBy().String(), EnableHighlight: req.GetEnableHighlight(),
	}
}

func toDomainSearchMerchantsReq(req *searchv1.SearchMerchantsReq) *domain.SearchMerchantsReq {
	verified := req.GetVerifiedOnly()
	return &domain.SearchMerchantsReq{
		Keyword: req.GetKeyword(), Page: req.GetPage(), PageSize: req.GetPageSize(),
		Region: req.GetRegion(), MinRating: req.GetMinRating(), Verified: &verified,
		SortBy: req.GetSortBy().String(),
	}
}

// ==================== Domain 鈫?Proto ====================

func toProtoProductList(products []domain.ProductSearchResult) []*searchv1.ProductSearchResult {
	res := make([]*searchv1.ProductSearchResult, len(products))
	for i, p := range products {
		res[i] = &searchv1.ProductSearchResult{
			Id: p.ID, Name: p.Name, Description: p.Description,
			Picture: p.Picture, SliderImgs: p.SliderImgs, Price: p.Price,
			Categories: p.Categories, InStock: p.InStock,
			MerchantId: p.MerchantID, MerchantName: p.MerchantName,
			Score: p.Score, NameHighlight: p.NameHighlight,
			DescriptionHighlight: p.DescriptionHighlight,
		}
	}
	return res
}

func toProtoMerchantList(merchants []domain.MerchantSearchResult) []*searchv1.MerchantSearchResult {
	res := make([]*searchv1.MerchantSearchResult, len(merchants))
	for i, m := range merchants {
		res[i] = &searchv1.MerchantSearchResult{
			Id: m.ID, Name: m.Name, Description: m.Description,
			Logo: m.Logo, Region: m.Region, Rating: m.Rating,
			SalesCount: m.SalesCount, ProductCount: m.ProductCount,
			Verified: m.Verified, Score: m.Score, NameHighlight: m.NameHighlight,
		}
	}
	return res
}

func toProtoSuggestionList(suggestions []domain.SearchSuggestion) []*searchv1.SearchSuggestion {
	res := make([]*searchv1.SearchSuggestion, len(suggestions))
	for i, s := range suggestions {
		source := searchv1.SuggestSource_NAME_MATCH
		switch s.Source {
		case "HISTORY":
			source = searchv1.SuggestSource_HISTORY
		case "HOT":
			source = searchv1.SuggestSource_HOT
		}
		res[i] = &searchv1.SearchSuggestion{
			Keyword: s.Keyword, Source: source, Count: s.Count,
		}
	}
	return res
}

func toProtoCategoryAggList(aggs []domain.CategoryAggregation) []*searchv1.CategoryAggregation {
	res := make([]*searchv1.CategoryAggregation, len(aggs))
	for i, a := range aggs {
		res[i] = &searchv1.CategoryAggregation{Category: a.Category, Count: a.Count}
	}
	return res
}

func toProtoPriceRangeAggList(aggs []domain.PriceRangeAggregation) []*searchv1.PriceRangeAggregation {
	res := make([]*searchv1.PriceRangeAggregation, len(aggs))
	for i, a := range aggs {
		res[i] = &searchv1.PriceRangeAggregation{
			MinPrice: a.MinPrice, MaxPrice: a.MaxPrice, Count: a.Count, Label: a.Label,
		}
	}
	return res
}
