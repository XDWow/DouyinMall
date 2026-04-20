package addtocart

type AddToCartResolveInput struct {
	ProductID      string
	ProductName    string
	ProductRef     string
	Spec           string
	Quantity       int
	CurrentProduct string
	CurrentSpec    string
	ProductList    []string
}

type ResolvedAddToCart struct {
	ProductID string
	Spec      string
	Quantity  int
}
