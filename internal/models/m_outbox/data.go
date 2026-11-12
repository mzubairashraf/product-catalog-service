// Package m_outbox is the persistence model of the outbox_events table.
package m_outbox

import (
	"encoding/json"
	"time"

	"cloud.google.com/go/spanner"
)

// EventStatus represents the processing status of an outbox event
type EventStatus string

const (
	EventStatusPending   EventStatus = "pending"
	EventStatusProcessed EventStatus = "processed"
	EventStatusFailed    EventStatus = "failed"
)

// Data represents an outbox event row in the database
type Data struct {
	EventID     string
	EventType   string
	AggregateID string
	Payload     string // JSON document
	Status      string
	CreatedAt   time.Time
	ProcessedAt spanner.NullTime
}

// Model provides type-safe operations for the outbox_events table
type Model struct {
	tableName string
}

// NewModel creates a new outbox events table model
func NewModel() *Model {
	return &Model{tableName: TableName}
}

// InsertMut creates an insert mutation for an outbox event. The payload
// column has Spanner's JSON type, so the already-serialised document is
// passed as raw JSON rather than as a string.
func (m *Model) InsertMut(data *Data) *spanner.Mutation {
	return spanner.Insert(
		m.tableName,
		AllColumns(),
		[]interface{}{
			data.EventID,
			data.EventType,
			data.AggregateID,
			spanner.NullJSON{Value: json.RawMessage(data.Payload), Valid: true},
			data.Status,
			data.CreatedAt,
			data.ProcessedAt,
		},
	)
}

// TableName returns the table name
func (m *Model) TableName() string {
	return m.tableName
}
