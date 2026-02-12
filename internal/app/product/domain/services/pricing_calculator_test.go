package services

import (
	"testing"
	"time"

	"product-catalog-service/internal/app/product/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPricingCalculator_CalculateEffectivePrice(t *testing.T) {
	calc := NewPricingCalculator()
	now := time.Now()

	t.Run("no discount - returns base price", func(t *testing.T) {
		basePrice, _ := domain.NewMoney(1999, 100)

		effective, err := calc.CalculateEffectivePrice(basePrice, nil, now)
		require.NoError(t, err)
		assert.True(t, effective.Equals(basePrice))
	})

	t.Run("active discount - returns discounted price", func(t *testing.T) {
		basePrice, _ := domain.NewMoney(1999, 100)
		discount, _ := domain.NewDiscount(20, now.Add(-1*time.Hour), now.Add(1*time.Hour))

		effective, err := calc.CalculateEffectivePrice(basePrice, discount, now)
		require.NoError(t, err)

		expected, _ := basePrice.ApplyDiscount(20)
		assert.True(t, effective.Equals(expected))
	})

	t.Run("inactive discount (future) - returns base price", func(t *testing.T) {
		basePrice, _ := domain.NewMoney(1999, 100)
		discount, _ := domain.NewDiscount(20, now.Add(1*time.Hour), now.Add(2*time.Hour))

		effective, err := calc.CalculateEffectivePrice(basePrice, discount, now)
		require.NoError(t, err)
		assert.True(t, effective.Equals(basePrice))
	})

	t.Run("inactive discount (expired) - returns base price", func(t *testing.T) {
		basePrice, _ := domain.NewMoney(1999, 100)
		discount, _ := domain.NewDiscount(20, now.Add(-2*time.Hour), now.Add(-1*time.Hour))

		effective, err := calc.CalculateEffectivePrice(basePrice, discount, now)
		require.NoError(t, err)
		assert.True(t, effective.Equals(basePrice))
	})

	t.Run("100% discount", func(t *testing.T) {
		basePrice, _ := domain.NewMoney(1999, 100)
		discount, _ := domain.NewDiscount(100, now, now.Add(1*time.Hour))

		effective, err := calc.CalculateEffectivePrice(basePrice, discount, now)
		require.NoError(t, err)

		assert.InDelta(t, 0.0, effective.Float64(), 0.001)
	})

	t.Run("50% discount", func(t *testing.T) {
		basePrice, _ := domain.NewMoney(10000, 100)
		discount, _ := domain.NewDiscount(50, now, now.Add(1*time.Hour))

		effective, err := calc.CalculateEffectivePrice(basePrice, discount, now)
		require.NoError(t, err)

		assert.InDelta(t, 50.0, effective.Float64(), 0.001)
	})
}
