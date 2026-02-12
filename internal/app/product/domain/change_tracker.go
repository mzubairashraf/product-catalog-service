package domain

// ChangeTracker tracks which fields have been modified in an aggregate
// This enables optimized database updates by only updating changed fields
type ChangeTracker struct {
	dirtyFields map[string]bool
}

// NewChangeTracker creates a new change tracker
func NewChangeTracker() *ChangeTracker {
	return &ChangeTracker{
		dirtyFields: make(map[string]bool),
	}
}

// MarkDirty marks a field as changed
func (ct *ChangeTracker) MarkDirty(field string) {
	ct.dirtyFields[field] = true
}

// Dirty checks if a field has been modified
func (ct *ChangeTracker) Dirty(field string) bool {
	return ct.dirtyFields[field]
}

// HasChanges returns true if any fields have been modified
func (ct *ChangeTracker) HasChanges() bool {
	return len(ct.dirtyFields) > 0
}

// Reset clears all dirty fields
func (ct *ChangeTracker) Reset() {
	ct.dirtyFields = make(map[string]bool)
}

// Field name constants for change tracking
const (
	FieldName        = "name"
	FieldDescription = "description"
	FieldCategory    = "category"
	FieldBasePrice   = "base_price"
	FieldDiscount    = "discount"
	FieldStatus      = "status"
)
