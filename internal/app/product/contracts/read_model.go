package contracts

import (
	"context"
	"time"
)

// ReadModel defines the contract for read-optimized product queries
// Read models bypass the domain layer for performance
type ReadModel interface {
	// GetProduct retrieves a product with its effective price
	GetProduct(ctx context.Context, productID string) (*ProductDTO, error)

	// ListProducts retrieves products with pagination and filtering
	ListProducts(ctx context.Context, req ListProductsRequest) (*ProductList, error)
}

// ProductDTO is a data transfer object for product queries
type ProductDTO struct {
	ProductID      string
	Name           string
	Description    string
	Category       string
	BasePrice      MoneyDTO
	Discount       *DiscountDTO
	EffectivePrice MoneyDTO
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// MoneyDTO represents money in queries
type MoneyDTO struct {
	Numerator   int64
	Denominator int64
	// Float64Value is for display purposes
	Float64Value float64
}

// DiscountDTO represents a discount in queries
type DiscountDTO struct {
	Percentage int
	StartDate  time.Time
	EndDate    time.Time
	IsActive   bool
}

// ListProductsRequest defines parameters for listing products
type ListProductsRequest struct {
	// Filtering
	Category string // Optional: filter by category
	Status   string // Optional: filter by status (active, inactive)

	// Pagination
	Limit  int
	Offset int
}

// ProductList is the result of a list query
type ProductList struct {
	Products []*ProductDTO
	HasMore  bool
}
