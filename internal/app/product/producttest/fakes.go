// Package producttest provides in-memory fakes of the product contracts for
// use-case tests. The fakes record what a use case asked for, so a test can
// assert on the mutations and outbox events that would have been committed.
package producttest

import (
	"context"
	"time"

	"cloud.google.com/go/spanner"

	"github.com/mzubairashraf/product-catalog-service/internal/app/product/contracts"
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/domain"
)

// Now is the fixed instant the fakes and fixtures use.
var Now = time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)

// Clock is a clock.Clock that always returns the same instant.
type Clock time.Time

func (c Clock) Now() time.Time { return time.Time(c) }

// Repo is an in-memory contracts.ProductRepository.
type Repo struct {
	Products map[string]*domain.Product
	Inserted []*domain.Product
	Updated  []*domain.Product
}

// NewRepo returns a repository holding the given products.
func NewRepo(products ...*domain.Product) *Repo {
	r := &Repo{Products: map[string]*domain.Product{}}
	for _, p := range products {
		r.Products[p.ID()] = p
	}
	return r
}

func (r *Repo) FindByID(_ context.Context, id string) (*domain.Product, error) {
	if p, ok := r.Products[id]; ok {
		return p, nil
	}
	return nil, domain.ErrProductNotFound
}

func (r *Repo) Exists(_ context.Context, id string) (bool, error) {
	_, ok := r.Products[id]
	return ok, nil
}

func (r *Repo) InsertMut(p *domain.Product) *spanner.Mutation {
	r.Inserted = append(r.Inserted, p)
	return spanner.Insert("products", []string{"product_id"}, []any{p.ID()})
}

func (r *Repo) UpdateMut(p *domain.Product) *spanner.Mutation {
	r.Updated = append(r.Updated, p)
	return spanner.Update("products", []string{"product_id"}, []any{p.ID()})
}

// Outbox is an in-memory contracts.OutboxRepository.
type Outbox struct {
	Events []contracts.OutboxEvent
}

func (o *Outbox) InsertMut(e contracts.OutboxEvent) *spanner.Mutation {
	o.Events = append(o.Events, e)
	return spanner.Insert("outbox", []string{"event_id"}, []any{e.EventID})
}

// EventTypes returns the recorded event types in order.
func (o *Outbox) EventTypes() []string {
	types := make([]string, 0, len(o.Events))
	for _, e := range o.Events {
		types = append(types, e.EventType)
	}
	return types
}

// Committer is a committer.Committer that records each applied plan and can
// be told to fail.
type Committer struct {
	Applied [][]*spanner.Mutation
	Err     error
}

func (c *Committer) Apply(_ context.Context, mutations []*spanner.Mutation) error {
	if c.Err != nil {
		return c.Err
	}
	c.Applied = append(c.Applied, mutations)
	return nil
}

// Product returns a stored product in the given status, priced at 100.00.
func Product(id string, status domain.ProductStatus, discount *domain.Discount) *domain.Product {
	price, err := domain.NewMoney(10000, 100)
	if err != nil {
		panic(err)
	}
	return domain.LoadProduct(id, "Widget", "A widget", "Tools", price, discount, status, Now, Now, nil)
}

// Discount returns a discount that is active at Now.
func Discount(percent int) *domain.Discount {
	d, err := domain.NewDiscount(percent, Now.Add(-time.Hour), Now.Add(24*time.Hour))
	if err != nil {
		panic(err)
	}
	return d
}

var (
	_ contracts.ProductRepository = (*Repo)(nil)
	_ contracts.OutboxRepository  = (*Outbox)(nil)
)
