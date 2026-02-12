package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDiscount(t *testing.T) {
	now := time.Now()

	t.Run("valid discount", func(t *testing.T) {
		discount, err := NewDiscount(20, now, now.Add(24*time.Hour))
		require.NoError(t, err)
		assert.NotNil(t, discount)
		assert.Equal(t, 20, discount.Percentage())
	})

	t.Run("zero percentage fails", func(t *testing.T) {
		_, err := NewDiscount(0, now, now.Add(24*time.Hour))
		assert.ErrorIs(t, err, ErrDiscountPercentInvalid)
	})

	t.Run("negative percentage fails", func(t *testing.T) {
		_, err := NewDiscount(-10, now, now.Add(24*time.Hour))
		assert.ErrorIs(t, err, ErrDiscountPercentInvalid)
	})

	t.Run("percentage over 100 fails", func(t *testing.T) {
		_, err := NewDiscount(101, now, now.Add(24*time.Hour))
		assert.ErrorIs(t, err, ErrDiscountPercentInvalid)
	})

	t.Run("end before start fails", func(t *testing.T) {
		_, err := NewDiscount(20, now, now.Add(-1*time.Hour))
		assert.ErrorIs(t, err, ErrInvalidDiscountPeriod)
	})

	t.Run("start equals end fails", func(t *testing.T) {
		_, err := NewDiscount(20, now, now)
		assert.ErrorIs(t, err, ErrInvalidDiscountPeriod)
	})

	t.Run("maximum percentage 100", func(t *testing.T) {
		discount, err := NewDiscount(100, now, now.Add(24*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 100, discount.Percentage())
	})

	t.Run("minimum percentage 1", func(t *testing.T) {
		discount, err := NewDiscount(1, now, now.Add(24*time.Hour))
		require.NoError(t, err)
		assert.Equal(t, 1, discount.Percentage())
	})
}

func TestDiscount_IsActive(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)
	discount, _ := NewDiscount(20, start, end)

	t.Run("active during period", func(t *testing.T) {
		checkTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
		assert.True(t, discount.IsActive(checkTime))
	})

	t.Run("active at start time", func(t *testing.T) {
		assert.True(t, discount.IsActive(start))
	})

	t.Run("not active at end time", func(t *testing.T) {
		assert.False(t, discount.IsActive(end))
	})

	t.Run("not active before start", func(t *testing.T) {
		checkTime := time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC)
		assert.False(t, discount.IsActive(checkTime))
	})

	t.Run("not active after end", func(t *testing.T) {
		checkTime := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
		assert.False(t, discount.IsActive(checkTime))
	})

	t.Run("active one nanosecond before end", func(t *testing.T) {
		checkTime := end.Add(-1 * time.Nanosecond)
		assert.True(t, discount.IsActive(checkTime))
	})
}

func TestDiscount_StartDate_EndDate(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC)
	discount, _ := NewDiscount(20, start, end)

	t.Run("start date matches", func(t *testing.T) {
		assert.Equal(t, start, discount.StartDate())
	})

	t.Run("end date matches", func(t *testing.T) {
		assert.Equal(t, end, discount.EndDate())
	})
}
