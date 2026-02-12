package list_products






















}	return q.readModel.ListProducts(ctx, req)func (q *Query) Execute(ctx context.Context, req contracts.ListProductsRequest) (*contracts.ProductList, error) {// Execute retrieves a list of products}	}		readModel: readModel,	return &Query{func NewQuery(readModel contracts.ReadModel) *Query {// NewQuery creates a new list products query}	readModel contracts.ReadModeltype Query struct {// Query handles listing products with filtering and pagination)	"product-catalog-service/internal/app/product/contracts"	"context"import (