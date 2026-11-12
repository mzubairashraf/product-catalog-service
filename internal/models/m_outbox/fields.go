package m_outbox

// Table and column names of the outbox_events table. They must match
// migrations/001_initial_schema.sql.
const (
	TableName = "outbox_events"

	EventID     = "event_id"
	EventType   = "event_type"
	AggregateID = "aggregate_id"
	Payload     = "payload"
	Status      = "status"
	CreatedAt   = "created_at"
	ProcessedAt = "processed_at"
)

// AllColumns returns the columns in the order InsertMut writes them.
func AllColumns() []string {
	return []string{EventID, EventType, AggregateID, Payload, Status, CreatedAt, ProcessedAt}
}
