package list_products

import (
	"context"

	"product-catalog-service/internal/app/product/contracts"
)

// Query handles listing products with filtering and pagination
type Query struct {
	readModel contracts.ReadModel
}

// NewQuery creates a new list products query
func NewQuery(readModel contracts.ReadModel) *Query {
	return &Query{
		readModel: readModel,
	}
}

// Request represents the input for listing products
type Request struct {
	Category string
	Status   string
	Limit    int
	Offset   int
}

// Execute retrieves a list of products
func (q *Query) Execute(ctx context.Context, req Request) (*contracts.ProductList, error) {
	return q.readModel.ListProducts(ctx, contracts.ListProductsRequest{
		Category: req.Category,
		Status:   req.Status,
		Limit:    req.Limit,
		Offset:   req.Offset,
	})
}
