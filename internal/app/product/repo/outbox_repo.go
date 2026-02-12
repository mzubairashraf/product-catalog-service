package repo

import (
	"encoding/json"
	"fmt"
	"product-catalog-service/internal/app/product/contracts"
	"product-catalog-service/internal/models/m_outbox"
	"time"

	"cloud.google.com/go/spanner"
	"github.com/google/uuid"
)

// OutboxRepo is the Spanner implementation for outbox events
type OutboxRepo struct {
	model *m_outbox.Model
}

// NewOutboxRepo creates a new outbox repository
func NewOutboxRepo() *OutboxRepo {
	return &OutboxRepo{
		model: m_outbox.NewModel(),
	}
}

// InsertMut returns a mutation for inserting an outbox event
func (r *OutboxRepo) InsertMut(event contracts.OutboxEvent) *spanner.Mutation {
	data := &m_outbox.Data{
		EventID:     event.EventID,
		EventType:   event.EventType,
		AggregateID: event.AggregateID,
		Payload:     event.Payload,
		Status:      string(m_outbox.EventStatusPending),
		CreatedAt:   event.CreatedAt,
		ProcessedAt: spanner.NullTime{}, // Not processed yet
	}

	return r.model.InsertMut(data)
}

// EnrichDomainEvent converts a domain event to an outbox event with metadata
// This function adds context and serialization logic
func EnrichDomainEvent(event interface{}, eventType, aggregateID string, now time.Time) (contracts.OutboxEvent, error) {
	// Serialize event payload to JSON
	payload, err := json.Marshal(event)
	if err != nil {
		return contracts.OutboxEvent{}, fmt.Errorf("failed to marshal event: %w", err)
	}

	return contracts.OutboxEvent{
		EventID:     uuid.New().String(),
		EventType:   eventType,
		AggregateID: aggregateID,
		Payload:     string(payload),
		CreatedAt:   now,
	}, nil
}

// Ensure OutboxRepo implements the interface
var _ contracts.OutboxRepository = (*OutboxRepo)(nil)
