package activate_product

import (
	"context"
	"fmt"

	"github.com/mzubairashraf/product-catalog-service/internal/app/product/contracts"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/repo"
	"github.com/mzubairashraf/product-catalog-service/internal/pkg/clock"
	"github.com/mzubairashraf/product-catalog-service/internal/pkg/committer"
)

// Request defines the input for activating a product
type Request struct {
	ProductID string
}

// Interactor implements the activate product use case
type Interactor struct {
	productRepo contracts.ProductRepository
	outboxRepo  contracts.OutboxRepository
	committer   committer.Committer
	clock       clock.Clock
}

// NewInteractor creates a new activate product interactor
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

// Execute activates a product
func (it *Interactor) Execute(ctx context.Context, req Request) error {
	// 1. Load aggregate
	product, err := it.productRepo.FindByID(ctx, req.ProductID)
	if err != nil {
		return fmt.Errorf("failed to find product: %w", err)
	}

	// 2. Apply business logic
	if err := product.Activate(it.clock.Now()); err != nil {
		return fmt.Errorf("failed to activate product: %w", err)
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
