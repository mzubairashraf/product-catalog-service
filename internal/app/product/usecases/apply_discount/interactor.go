package apply_discount

import (
	"context"
	"fmt"
	"time"

	"product-catalog-service/internal/app/product/contracts"
	"product-catalog-service/internal/app/product/domain"
	"product-catalog-service/internal/app/product/repo"
	"product-catalog-service/internal/pkg/clock"
	"product-catalog-service/internal/pkg/committer"
)

// Request defines the input for applying a discount
type Request struct {
	ProductID       string
	DiscountPercent int
	StartDate       time.Time
	EndDate         time.Time
}

// Interactor implements the apply discount use case
type Interactor struct {
	productRepo contracts.ProductRepository
	outboxRepo  contracts.OutboxRepository
	committer   committer.Committer
	clock       clock.Clock
}

// NewInteractor creates a new apply discount interactor
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

// Execute applies a discount to a product
func (it *Interactor) Execute(ctx context.Context, req Request) error {
	// 1. Load aggregate
	product, err := it.productRepo.FindByID(ctx, req.ProductID)
	if err != nil {
		return fmt.Errorf("failed to find product: %w", err)
	}

	// 2. Create discount value object
	discount, err := domain.NewDiscount(req.DiscountPercent, req.StartDate, req.EndDate)
	if err != nil {
		return fmt.Errorf("invalid discount: %w", err)
	}

	// 3. Apply business logic
	if err := product.ApplyDiscount(discount, it.clock.Now()); err != nil {
		return fmt.Errorf("failed to apply discount: %w", err)
	}

	// 4. Build commit plan
	plan := committer.NewPlan()

	// 5. Get mutations from repository
	if mut := it.productRepo.UpdateMut(product); mut != nil {
		plan.Add(mut)
	}

	// 6. Add outbox events
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

	// 7. Apply plan
	if err := it.committer.Apply(ctx, plan.Mutations()); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	return nil
}
