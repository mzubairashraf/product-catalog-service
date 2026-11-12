package services

import (
	"github.com/mzubairashraf/product-catalog-service/internal/app/product/domain"
	"time"
)

type Money = domain.Money
type Discount = domain.Discount

type PricingCalculator struct{}

func NewPricingCalculator() *PricingCalculator {
	return &PricingCalculator{}
}

func (pc *PricingCalculator) CalculateEffectivePrice(
	basePrice *Money,
	discount *Discount,
	now time.Time,
) (*Money, error) {
	if discount == nil || !discount.IsActive(now) {
		return basePrice, nil
	}
	return basePrice.ApplyDiscount(discount.Percentage())
}
