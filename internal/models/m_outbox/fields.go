package m_outbox

import (
	"cloud.google.com/go/spanner"
)

// Field constants for outbox table
const (
	TableName = "outbox"

	// Shorter aliases
	EventID     = "event_id"
	EventType   = "event_type"
	AggregateID = "aggregate_id"
	Payload     = "payload"
	CreatedAt   = "created_at"

	// Full field names
	FieldEventID     = "event_id"
	FieldEventType   = "event_type"
	FieldAggregateID = "aggregate_id"
	FieldPayload     = "payload"
	FieldCreatedAt   = "created_at"
)

// AllFields returns all field names
func AllFields() []string {
	return []string{
		FieldEventID,
		FieldEventType,
		FieldAggregateID,
		FieldPayload,
		FieldCreatedAt,
	}
}

// AllColumns is an alias for AllFields for backward compatibility
func AllColumns() []string {
	return AllFields()
}

// ToSpannerRow converts the outbox event to a Spanner row map
func ToSpannerRow(eventID, eventType, aggregateID, payload, createdAt string) map[string]interface{} {
	return map[string]interface{}{
		FieldEventID:     eventID,
		FieldEventType:   eventType,
		FieldAggregateID: aggregateID,
		FieldPayload:     payload,
		FieldCreatedAt:   createdAt,
	}
}

// InsertMutation creates a Spanner mutation for inserting an outbox event
func InsertMutation(eventID, eventType, aggregateID, payload, createdAt string) *spanner.Mutation {
	return spanner.Insert(
		"outbox",
		[]string{FieldEventID, FieldEventType, FieldAggregateID, FieldPayload, FieldCreatedAt},
		[]interface{}{eventID, eventType, aggregateID, payload, createdAt},
	)
}
