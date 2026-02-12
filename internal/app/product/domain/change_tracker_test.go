package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestChangeTracker(t *testing.T) {
	t.Run("new tracker has no changes", func(t *testing.T) {
		tracker := NewChangeTracker()

		assert.False(t, tracker.HasChanges())
		assert.False(t, tracker.Dirty(FieldName))
		assert.False(t, tracker.Dirty(FieldDescription))
		assert.False(t, tracker.Dirty(FieldCategory))
		assert.False(t, tracker.Dirty(FieldBasePrice))
		assert.False(t, tracker.Dirty(FieldDiscount))
		assert.False(t, tracker.Dirty(FieldStatus))
	})

	t.Run("mark single field dirty", func(t *testing.T) {
		tracker := NewChangeTracker()

		tracker.MarkDirty(FieldName)

		assert.True(t, tracker.HasChanges())
		assert.True(t, tracker.Dirty(FieldName))
		assert.False(t, tracker.Dirty(FieldDescription))
	})

	t.Run("mark multiple fields dirty", func(t *testing.T) {
		tracker := NewChangeTracker()

		tracker.MarkDirty(FieldName)
		tracker.MarkDirty(FieldDescription)
		tracker.MarkDirty(FieldBasePrice)

		assert.True(t, tracker.HasChanges())
		assert.True(t, tracker.Dirty(FieldName))
		assert.True(t, tracker.Dirty(FieldDescription))
		assert.True(t, tracker.Dirty(FieldBasePrice))
		assert.False(t, tracker.Dirty(FieldCategory))
	})

	t.Run("mark same field multiple times", func(t *testing.T) {
		tracker := NewChangeTracker()

		tracker.MarkDirty(FieldName)
		tracker.MarkDirty(FieldName)
		tracker.MarkDirty(FieldName)

		assert.True(t, tracker.HasChanges())
		assert.True(t, tracker.Dirty(FieldName))
	})

	t.Run("reset clears all changes", func(t *testing.T) {
		tracker := NewChangeTracker()

		tracker.MarkDirty(FieldName)
		tracker.MarkDirty(FieldDescription)
		tracker.MarkDirty(FieldStatus)

		assert.True(t, tracker.HasChanges())

		tracker.Reset()

		assert.False(t, tracker.HasChanges())
		assert.False(t, tracker.Dirty(FieldName))
		assert.False(t, tracker.Dirty(FieldDescription))
		assert.False(t, tracker.Dirty(FieldStatus))
	})

	t.Run("check non-existent field", func(t *testing.T) {
		tracker := NewChangeTracker()

		// Should return false for unknown field
		assert.False(t, tracker.Dirty("unknown_field"))
	})

	t.Run("all field constants", func(t *testing.T) {
		tracker := NewChangeTracker()

		// Mark all standard fields
		tracker.MarkDirty(FieldName)
		tracker.MarkDirty(FieldDescription)
		tracker.MarkDirty(FieldCategory)
		tracker.MarkDirty(FieldBasePrice)
		tracker.MarkDirty(FieldDiscount)
		tracker.MarkDirty(FieldStatus)

		// All should be dirty
		assert.True(t, tracker.Dirty(FieldName))
		assert.True(t, tracker.Dirty(FieldDescription))
		assert.True(t, tracker.Dirty(FieldCategory))
		assert.True(t, tracker.Dirty(FieldBasePrice))
		assert.True(t, tracker.Dirty(FieldDiscount))
		assert.True(t, tracker.Dirty(FieldStatus))
	})
}
