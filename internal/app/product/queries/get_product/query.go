package get_product






















}	return q.readModel.GetProduct(ctx, productID)func (q *Query) Execute(ctx context.Context, productID string) (*contracts.ProductDTO, error) {// Execute retrieves a product by ID}	}		readModel: readModel,	return &Query{func NewQuery(readModel contracts.ReadModel) *Query {// NewQuery creates a new get product query}	readModel contracts.ReadModeltype Query struct {// Query handles retrieving a single product)	"product-catalog-service/internal/app/product/contracts"	"context"import (