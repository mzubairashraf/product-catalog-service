package get_product

import (
	"context"

	"product-catalog-service/internal/app/product/contracts"
)

// Query handles retrieving a single product
type Query struct {
	readModel contracts.ReadModel
}

// NewQuery creates a new get product query
func NewQuery(readModel contracts.ReadModel) *Query {
	return &Query{
		readModel: readModel,
	}
}

// Request represents the input for getting a product
type Request struct {
	ProductID string
}

// Execute retrieves a product by ID
func (q *Query) Execute(ctx context.Context, req Request) (*contracts.ProductDTO, error) {
	return q.readModel.GetProduct(ctx, req.ProductID)
}
