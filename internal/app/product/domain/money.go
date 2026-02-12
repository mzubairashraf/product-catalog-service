package domain

import (
	"errors"
	"math/big"
)

// Money represents a monetary value using precise decimal arithmetic
// Internally stored as a rational number (numerator/denominator)
type Money struct {
	value *big.Rat
}

// NewMoney creates a new Money value from numerator and denominator
// For example: NewMoney(1999, 100) represents $19.99
func NewMoney(numerator, denominator int64) (*Money, error) {
	if denominator == 0 {
		return nil, errors.New("denominator cannot be zero")
	}

	rat := big.NewRat(numerator, denominator)

	// Validate that the resulting value is positive
	if rat.Sign() <= 0 {
		return nil, ErrInvalidPrice
	}

	return &Money{value: rat}, nil
}

// NewMoneyFromRat creates Money from a big.Rat
func NewMoneyFromRat(rat *big.Rat) (*Money, error) {
	if rat == nil || rat.Sign() <= 0 {
		return nil, ErrInvalidPrice
	}

	// Create a copy to avoid external mutations
	value := new(big.Rat).Set(rat)
	return &Money{value: value}, nil
}

// Value returns the underlying big.Rat value (read-only)
func (m *Money) Value() *big.Rat {
	// Return a copy to prevent external mutation
	return new(big.Rat).Set(m.value)
}

// Numerator returns the numerator of the rational number
func (m *Money) Numerator() int64 {
	return m.value.Num().Int64()
}

// Denominator returns the denominator of the rational number
func (m *Money) Denominator() int64 {
	return m.value.Denom().Int64()
}

// Float64 returns the monetary value as a float64 (for display purposes only)
func (m *Money) Float64() float64 {
	f, _ := m.value.Float64()
	return f
}

// ApplyDiscount applies a percentage discount and returns the discounted price
// discount should be between 0 and 100
func (m *Money) ApplyDiscount(discountPercent int) (*Money, error) {
	if discountPercent < 0 || discountPercent > 100 {
		return nil, ErrDiscountPercentInvalid
	}

	if discountPercent == 0 {
		return m, nil
	}

	// Calculate discount multiplier: (100 - discountPercent) / 100
	multiplier := big.NewRat(int64(100-discountPercent), 100)

	// Apply discount
	discountedValue := new(big.Rat).Mul(m.value, multiplier)

	return &Money{value: discountedValue}, nil
}

// Equals checks if two Money values are equal
func (m *Money) Equals(other *Money) bool {
	if other == nil {
		return false
	}
	return m.value.Cmp(other.value) == 0
}
