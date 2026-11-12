// Package usecases holds cross-cutting tests for the command interactors in
// the sub-packages. Each interactor follows the same shape (load, apply a
// domain rule, build one commit plan), so they are tested as one table.
package usecases

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/mzubairashraf/product-catalog-service/internal/app/product/domain"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/producttest"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/activate_product"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/apply_discount"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/create_product"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/deactivate_product"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/remove_discount"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/usecases/update_product"
)

const id = "prod-1"

var (
	ctx = context.Background()
	now = producttest.Now
)

// env is the set of fakes one use case runs against.
type env struct {
	repo      *producttest.Repo
	outbox    *producttest.Outbox
	committer *producttest.Committer
	clock     producttest.Clock
}

func newEnv(products ...*domain.Product) *env {
	return &env{
		repo:      producttest.NewRepo(products...),
		outbox:    &producttest.Outbox{},
		committer: &producttest.Committer{},
		clock:     producttest.Clock(now),
	}
}

func (e *env) create(req create_product.Request) error {
	_, err := create_product.NewInteractor(e.repo, e.outbox, e.committer, e.clock).Execute(ctx, req)
	return err
}

func (e *env) update(name, category string) error {
	return update_product.NewInteractor(e.repo, e.outbox, e.committer, e.clock).
		Execute(ctx, update_product.Request{ProductID: id, Name: name, Description: "new", Category: category})
}

func (e *env) activate() error {
	return activate_product.NewInteractor(e.repo, e.outbox, e.committer, e.clock).Execute(ctx, activate_product.Request{ProductID: id})
}

func (e *env) deactivate() error {
	return deactivate_product.NewInteractor(e.repo, e.outbox, e.committer, e.clock).Execute(ctx, deactivate_product.Request{ProductID: id})
}

func (e *env) applyDiscount(percent int, start, end time.Time) error {
	return apply_discount.NewInteractor(e.repo, e.outbox, e.committer, e.clock).
		Execute(ctx, apply_discount.Request{ProductID: id, DiscountPercent: percent, StartDate: start, EndDate: end})
}

func (e *env) removeDiscount() error {
	return remove_discount.NewInteractor(e.repo, e.outbox, e.committer, e.clock).Execute(ctx, remove_discount.Request{ProductID: id})
}

// Every command must either commit the aggregate change and its event in one
// plan, or commit nothing. This table checks both halves for each use case.
func TestCommands(t *testing.T) {
	active := func() *domain.Product { return producttest.Product(id, domain.ProductStatusActive, nil) }
	inactive := func() *domain.Product { return producttest.Product(id, domain.ProductStatusInactive, nil) }
	discounted := func() *domain.Product {
		return producttest.Product(id, domain.ProductStatusActive, producttest.Discount(10))
	}
	archived := func() *domain.Product {
		price, _ := domain.NewMoney(10000, 100)
		at := now
		return domain.LoadProduct(id, "Widget", "", "Tools", price, nil, domain.ProductStatusArchived, now, now, &at)
	}
	price, _ := domain.NewMoney(1999, 100)
	hourAgo, inAnHour := now.Add(-time.Hour), now.Add(time.Hour)

	tests := []struct {
		name      string
		given     func() *domain.Product // nil: empty repository
		run       func(e *env) error
		wantErr   error
		wantEvent string                     // event type committed on success
		check     func(t *testing.T, e *env) // extra assertions on success
	}{
		{name: "create",
			run: func(e *env) error {
				return e.create(create_product.Request{Name: "Widget", Description: "d", Category: "Tools", BasePrice: price})
			},
			wantEvent: "product.created",
			check: func(t *testing.T, e *env) {
				p := e.repo.Inserted[0]
				if p.Status() != domain.ProductStatusInactive || p.ID() == "" || !p.CreatedAt().Equal(now) {
					t.Errorf("new product: status=%s id=%q created=%s", p.Status(), p.ID(), p.CreatedAt())
				}
			}},
		{name: "create without a name",
			run:     func(e *env) error { return e.create(create_product.Request{Category: "Tools", BasePrice: price}) },
			wantErr: domain.ErrInvalidProductName},
		{name: "create without a category",
			run:     func(e *env) error { return e.create(create_product.Request{Name: "Widget", BasePrice: price}) },
			wantErr: domain.ErrInvalidCategory},

		{name: "update", given: active,
			run:       func(e *env) error { return e.update("Gadget", "Toys") },
			wantEvent: "product.updated",
			check: func(t *testing.T, e *env) {
				if p := e.repo.Updated[0]; p.Name() != "Gadget" || p.Category() != "Toys" {
					t.Errorf("updated product = %s / %s", p.Name(), p.Category())
				}
			}},
		{name: "update with an empty name", given: active,
			run: func(e *env) error { return e.update("", "Toys") }, wantErr: domain.ErrInvalidProductName},
		{name: "update an archived product", given: archived,
			run: func(e *env) error { return e.update("Gadget", "Toys") }, wantErr: domain.ErrProductAlreadyArchived},
		{name: "update a missing product",
			run: func(e *env) error { return e.update("Gadget", "Toys") }, wantErr: domain.ErrProductNotFound},

		{name: "activate", given: inactive,
			run: (*env).activate, wantEvent: "product.activated"},
		{name: "activate twice", given: active,
			run: (*env).activate, wantErr: domain.ErrProductAlreadyActive},
		{name: "activate an archived product", given: archived,
			run: (*env).activate, wantErr: domain.ErrProductAlreadyArchived},

		{name: "deactivate", given: active,
			run: (*env).deactivate, wantEvent: "product.deactivated"},
		{name: "deactivate an inactive product", given: inactive,
			run: (*env).deactivate, wantErr: domain.ErrProductNotActive},

		{name: "apply discount", given: active,
			run:       func(e *env) error { return e.applyDiscount(20, hourAgo, inAnHour) },
			wantEvent: "discount.applied",
			check: func(t *testing.T, e *env) {
				got := e.repo.Updated[0].EffectivePrice(now)
				if want, _ := domain.NewMoney(8000, 100); !got.Equals(want) {
					t.Errorf("effective price = %v, want 80.00", got.Float64())
				}
			}},
		{name: "apply discount over 100 percent", given: active,
			run: func(e *env) error { return e.applyDiscount(150, hourAgo, inAnHour) }, wantErr: domain.ErrDiscountPercentInvalid},
		{name: "apply discount of zero percent", given: active,
			run: func(e *env) error { return e.applyDiscount(0, hourAgo, inAnHour) }, wantErr: domain.ErrDiscountPercentInvalid},
		{name: "apply discount that ends before it starts", given: active,
			run: func(e *env) error { return e.applyDiscount(10, inAnHour, hourAgo) }, wantErr: domain.ErrInvalidDiscountPeriod},
		{name: "apply discount that has already ended", given: active,
			run: func(e *env) error { return e.applyDiscount(10, hourAgo.Add(-time.Hour), hourAgo) }, wantErr: domain.ErrInvalidDiscountPeriod},
		{name: "apply discount to an inactive product", given: inactive,
			run: func(e *env) error { return e.applyDiscount(10, hourAgo, inAnHour) }, wantErr: domain.ErrProductNotActive},
		{name: "apply a second discount", given: discounted,
			run: func(e *env) error { return e.applyDiscount(30, hourAgo, inAnHour) }, wantErr: domain.ErrDiscountAlreadyActive},

		{name: "remove discount", given: discounted,
			run: (*env).removeDiscount, wantEvent: "discount.removed"},
		{name: "remove a discount that is not there", given: active,
			run: (*env).removeDiscount, wantErr: domain.ErrNoActiveDiscount},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv()
			if tt.given != nil {
				e = newEnv(tt.given())
			}
			err := tt.run(e)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				if len(e.committer.Applied) != 0 {
					t.Errorf("a failed command committed %d plan(s)", len(e.committer.Applied))
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// One plan, holding exactly the aggregate mutation and its event.
			if len(e.committer.Applied) != 1 || len(e.committer.Applied[0]) != 2 {
				t.Fatalf("applied plans = %v, want one plan of two mutations", e.committer.Applied)
			}
			if got := e.outbox.EventTypes(); !slices.Equal(got, []string{tt.wantEvent}) {
				t.Errorf("events = %v, want [%s]", got, tt.wantEvent)
			}
			ev := e.outbox.Events[0]
			if ev.EventID == "" || ev.Payload == "" || !ev.CreatedAt.Equal(now) || (tt.given != nil && ev.AggregateID != id) {
				t.Errorf("outbox event = %+v", ev)
			}
			if tt.check != nil {
				tt.check(t, e)
			}
		})
	}
}

// If the commit fails, the caller must see the failure: there is no partial
// state to fall back on.
func TestCommitFailureIsReported(t *testing.T) {
	boom := errors.New("spanner unavailable")
	e := newEnv(producttest.Product(id, domain.ProductStatusInactive, nil))
	e.committer.Err = boom

	if err := e.activate(); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the commit error", err)
	}
}
