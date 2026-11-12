package create_product

import (
	"context"
	"fmt"

	"github.com/mzubairashraf/product-catalog-service/internal/app/product/contracts"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/domain"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/repo"
	"github.com/mzubairashraf/product-catalog-service/internal/pkg/clock"
	"github.com/mzubairashraf/product-catalog-service/internal/pkg/committer"
)

// Request defines the input for creating a product
type Request struct {
	Name        string
	Description string
	Category    string
	BasePrice   *domain.Money
}

// Response contains the created product ID
type Response struct {
	ProductID string
}

// Interactor implements the create product use case
type Interactor struct {
	productRepo contracts.ProductRepository
	outboxRepo  contracts.OutboxRepository
	committer   committer.Committer
	clock       clock.Clock
}

// NewInteractor creates a new create product interactor
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

// Execute creates a new product following the golden mutation pattern
func (it *Interactor) Execute(ctx context.Context, req Request) (*Response, error) {
	// 1. Create domain aggregate
	product, err := domain.NewProduct(req.Name, req.Description, req.Category, req.BasePrice, it.clock.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to create product: %w", err)
	}

	// 2. Build commit plan
	plan := committer.NewPlan()

	// 3. Get mutations from repository
	if mut := it.productRepo.InsertMut(product); mut != nil {
		plan.Add(mut)
	}

	// 4. Add outbox events
	for _, event := range product.DomainEvents() {
		outboxEvent, err := repo.EnrichDomainEvent(
			event,
			event.EventType(),
			product.ID(),
			it.clock.Now(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to enrich event: %w", err)
		}

		if mut := it.outboxRepo.InsertMut(outboxEvent); mut != nil {
			plan.Add(mut)
		}
	}

	// 5. Apply plan
	if err := it.committer.Apply(ctx, plan.Mutations()); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}

	return &Response{ProductID: product.ID()}, nil
}
