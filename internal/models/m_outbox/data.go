package m_outbox

import (
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
	Payload     string // JSON payload
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
	return &Model{
		tableName: TableName,
	}
}

// InsertMut creates an insert mutation for an outbox event
func (m *Model) InsertMut(data *Data) *spanner.Mutation {
	return spanner.Insert(
		m.tableName,
		AllColumns(),
		[]interface{}{
			data.EventID,
			data.EventType,
			data.AggregateID,
			data.Payload,
			data.Status,
			data.CreatedAt,
			data.ProcessedAt,
		},
	)
}

// UpdateMut creates an update mutation for an outbox event
func (m *Model) UpdateMut(eventID string, updates map[string]interface{}) *spanner.Mutation {
	columns := []string{EventID}
	values := []interface{}{eventID}

	for col, val := range updates {
		columns = append(columns, col)
		values = append(values, val)
	}

	return spanner.Update(m.tableName, columns, values)
}

// TableName returns the table name
func (m *Model) TableName() string {
	return m.tableName
}
