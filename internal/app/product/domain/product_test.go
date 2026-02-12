package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewProduct(t *testing.T) {
	name := "iPhone 15 Pro"
	desc := "Latest flagship smartphone"
	category := "Electronics"
	basePrice, _ := NewMoney(99900, 100)
	now := time.Now()

	t.Run("creates valid product", func(t *testing.T) {
		product, err := NewProduct(name, desc, category, basePrice, now)
		require.NoError(t, err)
		assert.NotNil(t, product)
		assert.NotEmpty(t, product.ID())
		assert.Equal(t, name, product.Name())
		assert.Equal(t, desc, product.Description())
		assert.Equal(t, category, product.Category())
		assert.True(t, product.BasePrice().Equals(basePrice))
		assert.Equal(t, ProductStatusActive, product.Status())
		assert.Nil(t, product.Discount())
	})

	t.Run("empty name fails", func(t *testing.T) {
		_, err := NewProduct("", desc, category, basePrice, now)
		assert.ErrorIs(t, err, ErrInvalidProductName)
	})

	t.Run("whitespace-only name fails", func(t *testing.T) {
		_, err := NewProduct("   ", desc, category, basePrice, now)
		assert.ErrorIs(t, err, ErrInvalidProductName)
	})

	t.Run("empty category fails", func(t *testing.T) {
		_, err := NewProduct(name, desc, "", basePrice, now)
		assert.ErrorIs(t, err, ErrInvalidCategory)
	})

	t.Run("empty description is allowed", func(t *testing.T) {
		product, err := NewProduct(name, "", category, basePrice, now)
		require.NoError(t, err)
		assert.Equal(t, "", product.Description())
	})

	t.Run("sets status to Active by default", func(t *testing.T) {
		product, _ := NewProduct(name, desc, category, basePrice, now)
		assert.Equal(t, ProductStatusActive, product.Status())
	})

	t.Run("emits ProductCreated event", func(t *testing.T) {
		product, _ := NewProduct(name, desc, category, basePrice, now)
		events := product.DomainEvents()
		require.Len(t, events, 1)

		event, ok := events[0].(ProductCreatedEvent)
		require.True(t, ok)
		assert.Equal(t, product.ID(), event.AggregateID())
		assert.Equal(t, name, event.Name)
		assert.Equal(t, category, event.Category)
	})
}

func TestProduct_Update(t *testing.T) {
	product := createTestProduct(t)
	now := time.Now()

	t.Run("updates name, description, category", func(t *testing.T) {
		newName := "iPhone 15 Pro Max"
		newDesc := "Updated description"
		newCategory := "Premium Electronics"

		err := product.Update(newName, newDesc, newCategory, now)
		require.NoError(t, err)

		assert.Equal(t, newName, product.Name())
		assert.Equal(t, newDesc, product.Description())
		assert.Equal(t, newCategory, product.Category())
	})

	t.Run("empty name fails", func(t *testing.T) {
		product := createTestProduct(t)
		err := product.Update("", "desc", "cat", now)
		assert.ErrorIs(t, err, ErrInvalidProductName)
	})

	t.Run("empty category fails", func(t *testing.T) {
		product := createTestProduct(t)
		err := product.Update("name", "desc", "", now)
		assert.ErrorIs(t, err, ErrInvalidCategory)
	})

	t.Run("tracks changed fields", func(t *testing.T) {
		product := createTestProduct(t)

		err := product.Update("New Name", product.Description(), "New Category", now)
		require.NoError(t, err)

		assert.True(t, product.Changes().Dirty(FieldName))
		assert.True(t, product.Changes().Dirty(FieldCategory))
		assert.False(t, product.Changes().Dirty(FieldDescription))
	})
}

func TestProduct_ApplyDiscount(t *testing.T) {
	now := time.Now()

	t.Run("applies discount successfully", func(t *testing.T) {
		product := createTestProduct(t)
		discount, _ := NewDiscount(20, now.Add(-1*time.Hour), now.Add(24*time.Hour))

		err := product.ApplyDiscount(discount, now)
		require.NoError(t, err)
		assert.NotNil(t, product.Discount())
		assert.Equal(t, 20, product.Discount().Percentage())
	})

	t.Run("fails when discount already active", func(t *testing.T) {
		product := createTestProduct(t)
		discount1, _ := NewDiscount(20, now.Add(-1*time.Hour), now.Add(24*time.Hour))
		discount2, _ := NewDiscount(30, now.Add(-1*time.Hour), now.Add(48*time.Hour))

		err := product.ApplyDiscount(discount1, now)
		require.NoError(t, err)

		err = product.ApplyDiscount(discount2, now)
		assert.ErrorIs(t, err, ErrDiscountAlreadyActive)
	})

	t.Run("tracks discount change", func(t *testing.T) {
		product := createTestProduct(t)

		discount, _ := NewDiscount(20, now.Add(-1*time.Hour), now.Add(24*time.Hour))
		err := product.ApplyDiscount(discount, now)
		require.NoError(t, err)

		assert.True(t, product.Changes().Dirty(FieldDiscount))
	})
}

func TestProduct_RemoveDiscount(t *testing.T) {
	now := time.Now()

	t.Run("removes active discount", func(t *testing.T) {
		product := createTestProduct(t)
		discount, _ := NewDiscount(20, now.Add(-1*time.Hour), now.Add(24*time.Hour))
		product.ApplyDiscount(discount, now)

		err := product.RemoveDiscount(now)
		require.NoError(t, err)
		assert.Nil(t, product.Discount())
	})

	t.Run("fails when no discount active", func(t *testing.T) {
		product := createTestProduct(t)

		err := product.RemoveDiscount(now)
		assert.ErrorIs(t, err, ErrNoActiveDiscount)
	})

	t.Run("tracks discount removal", func(t *testing.T) {
		product := createTestProduct(t)
		discount, _ := NewDiscount(20, now.Add(-1*time.Hour), now.Add(24*time.Hour))
		product.ApplyDiscount(discount, now)

		err := product.RemoveDiscount(now)
		require.NoError(t, err)

		assert.True(t, product.Changes().Dirty(FieldDiscount))
	})
}

func TestProduct_Deactivate(t *testing.T) {
	now := time.Now()

	t.Run("deactivates active product", func(t *testing.T) {
		product := createTestProduct(t)

		err := product.Deactivate(now)
		require.NoError(t, err)
		assert.Equal(t, ProductStatusInactive, product.Status())
	})

	t.Run("fails when already inactive", func(t *testing.T) {
		product := createTestProduct(t)
		product.Deactivate(now)

		err := product.Deactivate(now)
		assert.ErrorIs(t, err, ErrProductNotActive)
	})

	t.Run("tracks status change", func(t *testing.T) {
		product := createTestProduct(t)

		err := product.Deactivate(now)
		require.NoError(t, err)

		assert.True(t, product.Changes().Dirty(FieldStatus))
	})
}

func TestProduct_Activate(t *testing.T) {
	now := time.Now()

	t.Run("activates inactive product", func(t *testing.T) {
		product := createTestProduct(t)
		product.Deactivate(now)

		err := product.Activate(now)
		require.NoError(t, err)
		assert.Equal(t, ProductStatusActive, product.Status())
	})

	t.Run("fails when already active", func(t *testing.T) {
		product := createTestProduct(t)

		err := product.Activate(now)
		assert.ErrorIs(t, err, ErrProductAlreadyActive)
	})

	t.Run("tracks status change", func(t *testing.T) {
		product := createTestProduct(t)
		product.Deactivate(now)

		err := product.Activate(now)
		require.NoError(t, err)

		assert.True(t, product.Changes().Dirty(FieldStatus))
	})
}

func TestProduct_EffectivePrice(t *testing.T) {
	now := time.Now()

	t.Run("returns base price with no discount", func(t *testing.T) {
		product := createTestProduct(t)

		effectivePrice := product.EffectivePrice(now)
		assert.True(t, effectivePrice.Equals(product.BasePrice()))
	})

	t.Run("returns discounted price with active discount", func(t *testing.T) {
		product := createTestProduct(t)
		basePrice, _ := NewMoney(10000, 100)
		product, _ = NewProduct("Test", "Desc", "Cat", basePrice, now)

		discount, _ := NewDiscount(20, now.Add(-1*time.Hour), now.Add(1*time.Hour))
		product.ApplyDiscount(discount, now)

		effectivePrice := product.EffectivePrice(now)
		expected, _ := basePrice.ApplyDiscount(20)
		assert.True(t, effectivePrice.Equals(expected))
	})
}

// Helper function to create a test product
func createTestProduct(t *testing.T) *Product {
	name := "Test Product"
	desc := "Test Description"
	category := "Test Category"
	basePrice, _ := NewMoney(10000, 100)
	now := time.Now()

	product, err := NewProduct(name, desc, category, basePrice, now)
	require.NoError(t, err)
	return product
}
