package domain

import "time"

// DomainEvent is a marker interface for all domain events
type DomainEvent interface {
	EventType() string
	AggregateID() string
	OccurredAt() time.Time
}

// BaseEvent provides common fields for all events
type BaseEvent struct {
	aggregateID string
	occurredAt  time.Time
}

func (e BaseEvent) AggregateID() string {
	return e.aggregateID
}

func (e BaseEvent) OccurredAt() time.Time {
	return e.occurredAt
}

// ProductCreatedEvent is published when a product is created
type ProductCreatedEvent struct {
	BaseEvent
	Name        string
	Description string
	Category    string
	BasePrice   *Money
}

func (e ProductCreatedEvent) EventType() string {
	return "product.created"
}

// ProductUpdatedEvent is published when product details change
type ProductUpdatedEvent struct {
	BaseEvent
	Name        string
	Description string
	Category    string
}

func (e ProductUpdatedEvent) EventType() string {
	return "product.updated"
}

// ProductActivatedEvent is published when a product is activated
type ProductActivatedEvent struct {
	BaseEvent
}

func (e ProductActivatedEvent) EventType() string {
	return "product.activated"
}

// ProductDeactivatedEvent is published when a product is deactivated
type ProductDeactivatedEvent struct {
	BaseEvent
}

func (e ProductDeactivatedEvent) EventType() string {
	return "product.deactivated"
}

// ProductArchivedEvent is published when a product is archived
type ProductArchivedEvent struct {
	BaseEvent
}

func (e ProductArchivedEvent) EventType() string {
	return "product.archived"
}

// DiscountAppliedEvent is published when a discount is applied
type DiscountAppliedEvent struct {
	BaseEvent
	DiscountPercent int
	StartDate       time.Time
	EndDate         time.Time
	EffectivePrice  *Money
}

func (e DiscountAppliedEvent) EventType() string {
	return "discount.applied"
}

// DiscountRemovedEvent is published when a discount is removed
type DiscountRemovedEvent struct {
	BaseEvent
}

func (e DiscountRemovedEvent) EventType() string {
	return "discount.removed"
}
