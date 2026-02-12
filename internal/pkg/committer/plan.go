package committer

import (
	"context"

	"cloud.google.com/go/spanner"
)

// Committer provides transaction execution for the golden mutation pattern
// It applies mutation plans atomically
type Committer interface {
	Apply(ctx context.Context, mutations []*spanner.Mutation) error
}

// SpannerCommitter applies mutations to Spanner
type SpannerCommitter struct {
	client *spanner.Client
}

// NewSpannerCommitter creates a new Spanner committer
func NewSpannerCommitter(client *spanner.Client) *SpannerCommitter {
	return &SpannerCommitter{
		client: client,
	}
}

// Apply executes a list of mutations atomically in a single transaction
func (c *SpannerCommitter) Apply(ctx context.Context, mutations []*spanner.Mutation) error {
	if len(mutations) == 0 {
		return nil // No mutations to apply
	}

	_, err := c.client.Apply(ctx, mutations)
	return err
}

// Plan is a simple mutation builder following the commitplan pattern
// In a production environment with commitplan library, this would be replaced
type Plan struct {
	mutations []*spanner.Mutation
}

// NewPlan creates a new mutation plan
func NewPlan() *Plan {
	return &Plan{
		mutations: make([]*spanner.Mutation, 0),
	}
}

// Add adds a mutation to the plan
func (p *Plan) Add(mutation *spanner.Mutation) {
	if mutation != nil {
		p.mutations = append(p.mutations, mutation)
	}
}

// Mutations returns all mutations in the plan
func (p *Plan) Mutations() []*spanner.Mutation {
	return p.mutations
}

// HasMutations returns true if the plan contains any mutations
func (p *Plan) HasMutations() bool {
	return len(p.mutations) > 0
}
