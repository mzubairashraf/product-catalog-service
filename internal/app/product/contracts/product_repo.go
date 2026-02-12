package contracts

import (
	"context"

	"product-catalog-service/internal/app/product/domain"

	"cloud.google.com/go/spanner"
)

// ProductRepository defines the contract for product persistence
// Repository implementations must return mutations, not apply them
// This follows the Golden Mutation Pattern
type ProductRepository interface {
	// FindByID retrieves a product by ID for write operations
	// Returns ErrProductNotFound if not found
	FindByID(ctx context.Context, productID string) (*domain.Product, error)

	// InsertMut returns a mutation for inserting a new product
	// Returns nil if no mutation is needed
	InsertMut(product *domain.Product) *spanner.Mutation

	// UpdateMut returns a mutation for updating an existing product
	// Only updates fields marked as dirty in the change tracker
	// Returns nil if no changes detected
	UpdateMut(product *domain.Product) *spanner.Mutation

	// Exists checks if a product exists
	Exists(ctx context.Context, productID string) (bool, error)
}

// OutboxRepository defines the contract for outbox event persistence
type OutboxRepository interface {
	// InsertMut returns a mutation for inserting an outbox event
	InsertMut(event OutboxEvent) *spanner.Mutation
}

// OutboxEvent represents an enriched domain event ready for persistence
type OutboxEvent struct {
	EventID     string
	EventType   string
	AggregateID string
	Payload     string // JSON-serialized payload
	CreatedAt   string
}
