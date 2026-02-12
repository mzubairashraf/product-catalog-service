package update_product

import (
	"context"
	"fmt"

	"product-catalog-service/internal/app/product/contracts"
	"product-catalog-service/internal/app/product/repo"
	"product-catalog-service/internal/pkg/clock"
	"product-catalog-service/internal/pkg/committer"
)

// Request defines the input for updating a product
type Request struct {
	ProductID   string
	Name        string
	Description string
	Category    string
}

// Interactor implements the update product use case
type Interactor struct {
	productRepo contracts.ProductRepository
	outboxRepo  contracts.OutboxRepository
	committer   committer.Committer
	clock       clock.Clock
}

// NewInteractor creates a new update product interactor
func NewInteractor(
	productRepo contracts.ProductRepository,
	outboxRepo contracts.OutboxRepository,
	committer committer.Committer,
	clock clock.Clock,
) *Interactor {
	return &Interactor{
		productRepo: productRepo,
		outboxRepo:  outboxRepo,
		committer:   committer,
		clock:       clock,
	}
}

// Execute updates a product's details
func (it *Interactor) Execute(ctx context.Context, req Request) error {
	// 1. Load aggregate
	product, err := it.productRepo.FindByID(ctx, req.ProductID)
	if err != nil {
		return fmt.Errorf("failed to find product: %w", err)
	}

	// 2. Apply business logic
	if err := product.Update(req.Name, req.Description, req.Category, it.clock.Now()); err != nil {
		return fmt.Errorf("failed to update product: %w", err)
	}

	// 3. Build commit plan
	plan := committer.NewPlan()

	// 4. Get mutations from repository
	if mut := it.productRepo.UpdateMut(product); mut != nil {
		plan.Add(mut)
	}

	// 5. Add outbox events
	for _, event := range product.DomainEvents() {
		outboxEvent, err := repo.EnrichDomainEvent(
			event,
			event.EventType(),
			product.ID(),
			it.clock.Now(),
		)
		if err != nil {
			return fmt.Errorf("failed to enrich event: %w", err)
		}

		if mut := it.outboxRepo.InsertMut(outboxEvent); mut != nil {
			plan.Add(mut)
		}
	}

	// 6. Apply plan
	if err := it.committer.Apply(ctx, plan.Mutations()); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	return nil
}
