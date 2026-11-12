package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// ProductStatus represents the lifecycle state of a product
type ProductStatus string

const (
	ProductStatusActive   ProductStatus = "active"
	ProductStatusInactive ProductStatus = "inactive"
	ProductStatusArchived ProductStatus = "archived"
)

// Product is the aggregate root for product management
// It encapsulates all business rules and ensures data consistency
type Product struct {
	// Identity
	id string

	// Attributes
	name        string
	description string
	category    string
	basePrice   *Money
	discount    *Discount
	status      ProductStatus

	// Timestamps
	createdAt  time.Time
	updatedAt  time.Time
	archivedAt *time.Time

	// Infrastructure
	changes *ChangeTracker
	events  []DomainEvent
}

// NewProduct creates a new product with validation
// This is a factory method that ensures the product is created in a valid state
func NewProduct(name, description, category string, basePrice *Money, now time.Time) (*Product, error) {
	if strings.TrimSpace(name) == "" {
		return nil, ErrInvalidProductName
	}
	if strings.TrimSpace(category) == "" {
		return nil, ErrInvalidCategory
	}
	if basePrice == nil {
		return nil, ErrInvalidPrice
	}

	id := uuid.New().String()

	p := &Product{
		id:          id,
		name:        name,
		description: description,
		category:    category,
		basePrice:   basePrice,
		status:      ProductStatusInactive, // New products start as inactive
		createdAt:   now,
		updatedAt:   now,
		changes:     NewChangeTracker(),
		events:      []DomainEvent{},
	}

	// Capture creation event
	p.events = append(p.events, &ProductCreatedEvent{
		BaseEvent: BaseEvent{
			aggregateID: p.id,
			occurredAt:  now,
		},
		Name:        name,
		Description: description,
		Category:    category,
		BasePrice:   basePrice,
	})

	return p, nil
}

// LoadProduct reconstructs a product from persistence (no events generated)
func LoadProduct(
	id, name, description, category string,
	basePrice *Money,
	discount *Discount,
	status ProductStatus,
	createdAt, updatedAt time.Time,
	archivedAt *time.Time,
) *Product {
	return &Product{
		id:          id,
		name:        name,
		description: description,
		category:    category,
		basePrice:   basePrice,
		discount:    discount,
		status:      status,
		createdAt:   createdAt,
		updatedAt:   updatedAt,
		archivedAt:  archivedAt,
		changes:     NewChangeTracker(),
		events:      []DomainEvent{},
	}
}

// === Getters (Read-only access to aggregate state) ===

func (p *Product) ID() string {
	return p.id
}

func (p *Product) Name() string {
	return p.name
}

func (p *Product) Description() string {
	return p.description
}

func (p *Product) Category() string {
	return p.category
}

func (p *Product) BasePrice() *Money {
	return p.basePrice
}

func (p *Product) Discount() *Discount {
	return p.discount
}

func (p *Product) Status() ProductStatus {
	return p.status
}

func (p *Product) CreatedAt() time.Time {
	return p.createdAt
}

func (p *Product) UpdatedAt() time.Time {
	return p.updatedAt
}

func (p *Product) ArchivedAt() *time.Time {
	return p.archivedAt
}

func (p *Product) Changes() *ChangeTracker {
	return p.changes
}

func (p *Product) DomainEvents() []DomainEvent {
	return p.events
}

// === Business Methods (Encapsulated business logic) ===

// Update modifies product details
func (p *Product) Update(name, description, category string, now time.Time) error {
	if p.status == ProductStatusArchived {
		return ErrProductAlreadyArchived
	}

	if strings.TrimSpace(name) == "" {
		return ErrInvalidProductName
	}
	if strings.TrimSpace(category) == "" {
		return ErrInvalidCategory
	}

	changed := false

	if p.name != name {
		p.name = name
		p.changes.MarkDirty(FieldName)
		changed = true
	}

	if p.description != description {
		p.description = description
		p.changes.MarkDirty(FieldDescription)
		changed = true
	}

	if p.category != category {
		p.category = category
		p.changes.MarkDirty(FieldCategory)
		changed = true
	}

	if changed {
		p.updatedAt = now
		p.events = append(p.events, &ProductUpdatedEvent{
			BaseEvent: BaseEvent{
				aggregateID: p.id,
				occurredAt:  now,
			},
			Name:        name,
			Description: description,
			Category:    category,
		})
	}

	return nil
}

// Activate activates an inactive product
func (p *Product) Activate(now time.Time) error {
	if p.status == ProductStatusArchived {
		return ErrProductAlreadyArchived
	}
	if p.status == ProductStatusActive {
		return ErrProductAlreadyActive
	}

	p.status = ProductStatusActive
	p.updatedAt = now
	p.changes.MarkDirty(FieldStatus)

	p.events = append(p.events, &ProductActivatedEvent{
		BaseEvent: BaseEvent{
			aggregateID: p.id,
			occurredAt:  now,
		},
	})

	return nil
}

// Deactivate deactivates an active product
func (p *Product) Deactivate(now time.Time) error {
	if p.status == ProductStatusArchived {
		return ErrProductAlreadyArchived
	}
	if p.status == ProductStatusInactive {
		return ErrProductNotActive
	}

	p.status = ProductStatusInactive
	p.updatedAt = now
	p.changes.MarkDirty(FieldStatus)

	p.events = append(p.events, &ProductDeactivatedEvent{
		BaseEvent: BaseEvent{
			aggregateID: p.id,
			occurredAt:  now,
		},
	})

	return nil
}

// Archive soft-deletes a product
func (p *Product) Archive(now time.Time) error {
	if p.status == ProductStatusArchived {
		return ErrProductAlreadyArchived
	}

	p.status = ProductStatusArchived
	p.updatedAt = now
	p.archivedAt = &now
	p.changes.MarkDirty(FieldStatus)

	p.events = append(p.events, &ProductArchivedEvent{
		BaseEvent: BaseEvent{
			aggregateID: p.id,
			occurredAt:  now,
		},
	})

	return nil
}

// ApplyDiscount applies a discount to the product
func (p *Product) ApplyDiscount(discount *Discount, now time.Time) error {
	if p.status != ProductStatusActive {
		return ErrProductNotActive
	}

	if !discount.IsValidAt(now) {
		return ErrInvalidDiscountPeriod
	}

	if p.discount != nil && p.discount.IsActive(now) {
		return ErrDiscountAlreadyActive
	}

	p.discount = discount
	p.updatedAt = now
	p.changes.MarkDirty(FieldDiscount)

	// Calculate effective price for the event
	effectivePrice, _ := p.basePrice.ApplyDiscount(discount.Percentage())

	p.events = append(p.events, &DiscountAppliedEvent{
		BaseEvent: BaseEvent{
			aggregateID: p.id,
			occurredAt:  now,
		},
		DiscountPercent: discount.Percentage(),
		StartDate:       discount.StartDate(),
		EndDate:         discount.EndDate(),
		EffectivePrice:  effectivePrice,
	})

	return nil
}

// RemoveDiscount removes the current discount
func (p *Product) RemoveDiscount(now time.Time) error {
	if p.discount == nil {
		return ErrNoActiveDiscount
	}

	p.discount = nil
	p.updatedAt = now
	p.changes.MarkDirty(FieldDiscount)

	p.events = append(p.events, &DiscountRemovedEvent{
		BaseEvent: BaseEvent{
			aggregateID: p.id,
			occurredAt:  now,
		},
	})

	return nil
}

// EffectivePrice calculates the current price including any active discount
func (p *Product) EffectivePrice(now time.Time) *Money {
	if p.discount != nil && p.discount.IsActive(now) {
		discountedPrice, _ := p.basePrice.ApplyDiscount(p.discount.Percentage())
		return discountedPrice
	}
	return p.basePrice
}
