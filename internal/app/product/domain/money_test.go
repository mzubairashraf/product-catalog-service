package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMoney(t *testing.T) {
	t.Run("valid positive amount", func(t *testing.T) {
		money, err := NewMoney(1999, 100)
		require.NoError(t, err)
		assert.NotNil(t, money)
		assert.InDelta(t, 19.99, money.Float64(), 0.001)
	})

	t.Run("zero amount fails validation", func(t *testing.T) {
		_, err := NewMoney(0, 100)
		assert.ErrorIs(t, err, ErrInvalidPrice)
	})

	t.Run("negative amount fails validation", func(t *testing.T) {
		_, err := NewMoney(-100, 100)
		assert.ErrorIs(t, err, ErrInvalidPrice)
	})

	t.Run("zero denominator fails validation", func(t *testing.T) {
		_, err := NewMoney(100, 0)
		assert.Error(t, err)
	})

	t.Run("negative denominator fails validation", func(t *testing.T) {
		_, err := NewMoney(100, -100)
		assert.ErrorIs(t, err, ErrInvalidPrice)
	})

	t.Run("large amount", func(t *testing.T) {
		money, err := NewMoney(999999999, 100)
		require.NoError(t, err)
		assert.InDelta(t, 9999999.99, money.Float64(), 0.01)
	})
}

func TestMoney_Float64(t *testing.T) {
	t.Run("converts to float64 correctly", func(t *testing.T) {
		money, _ := NewMoney(10000, 100)
		assert.InDelta(t, 100.0, money.Float64(), 0.001)
	})

	t.Run("handles fractional amounts", func(t *testing.T) {
		money, _ := NewMoney(1234, 100)
		assert.InDelta(t, 12.34, money.Float64(), 0.001)
	})
}

func TestMoney_Numerator_Denominator(t *testing.T) {
	t.Run("returns correct numerator and denominator", func(t *testing.T) {
		money, _ := NewMoney(1999, 100)
		assert.Equal(t, int64(1999), money.Numerator())
		assert.Equal(t, int64(100), money.Denominator())
	})
}

func TestMoney_ApplyDiscount(t *testing.T) {
	t.Run("apply 20 percent discount", func(t *testing.T) {
		money, _ := NewMoney(10000, 100) // 100.00

		result, err := money.ApplyDiscount(20)
		require.NoError(t, err)
		assert.InDelta(t, 80.0, result.Float64(), 0.001)
	})

	t.Run("apply 50 percent discount", func(t *testing.T) {
		money, _ := NewMoney(10000, 100)

		result, err := money.ApplyDiscount(50)
		require.NoError(t, err)
		assert.InDelta(t, 50.0, result.Float64(), 0.001)
	})

	t.Run("apply 0 percent discount returns same", func(t *testing.T) {
		money, _ := NewMoney(10000, 100)

		result, err := money.ApplyDiscount(0)
		require.NoError(t, err)
		assert.True(t, money.Equals(result))
	})

	t.Run("apply 100 percent discount", func(t *testing.T) {
		money, _ := NewMoney(10000, 100)

		result, err := money.ApplyDiscount(100)
		require.NoError(t, err)
		assert.InDelta(t, 0.0, result.Float64(), 0.001)
	})

	t.Run("negative discount percentage fails", func(t *testing.T) {
		money, _ := NewMoney(10000, 100)

		_, err := money.ApplyDiscount(-10)
		assert.ErrorIs(t, err, ErrDiscountPercentInvalid)
	})

	t.Run("discount percentage over 100 fails", func(t *testing.T) {
		money, _ := NewMoney(10000, 100)

		_, err := money.ApplyDiscount(101)
		assert.ErrorIs(t, err, ErrDiscountPercentInvalid)
	})
}

func TestMoney_Equals(t *testing.T) {
	t.Run("equal amounts", func(t *testing.T) {
		m1, _ := NewMoney(1000, 100)
		m2, _ := NewMoney(1000, 100)

		assert.True(t, m1.Equals(m2))
	})

	t.Run("different amounts", func(t *testing.T) {
		m1, _ := NewMoney(1000, 100)
		m2, _ := NewMoney(1001, 100)

		assert.False(t, m1.Equals(m2))
	})

	t.Run("equivalent rational values", func(t *testing.T) {
		m1, _ := NewMoney(100, 10)   // 10.0
		m2, _ := NewMoney(1000, 100) // 10.0

		assert.True(t, m1.Equals(m2))
	})

	t.Run("nil comparison", func(t *testing.T) {
		m1, _ := NewMoney(1000, 100)

		assert.False(t, m1.Equals(nil))
	})
}
