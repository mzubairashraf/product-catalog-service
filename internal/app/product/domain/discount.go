package domain

import "time"

// Discount represents a time-bound percentage discount
// This is a value object - immutable and validated on creation
type Discount struct {
	percentage int
	startDate  time.Time
	endDate    time.Time
}

// NewDiscount creates a new discount with validation
func NewDiscount(percentage int, startDate, endDate time.Time) (*Discount, error) {
	if percentage < 0 || percentage > 100 {
		return nil, ErrDiscountPercentInvalid
	}

	if endDate.Before(startDate) {
		return nil, ErrInvalidDiscountPeriod
	}

	return &Discount{
		percentage: percentage,
		startDate:  startDate,
		endDate:    endDate,
	}, nil
}

// Percentage returns the discount percentage
func (d *Discount) Percentage() int {
	return d.percentage
}

// StartDate returns the start date of the discount period
func (d *Discount) StartDate() time.Time {
	return d.startDate
}

// EndDate returns the end date of the discount period
func (d *Discount) EndDate() time.Time {
	return d.endDate
}

// IsValidAt checks if the discount is valid at the given time
func (d *Discount) IsValidAt(now time.Time) bool {
	return !now.Before(d.startDate) && !now.After(d.endDate)
}

// IsActive checks if the discount is currently active
func (d *Discount) IsActive(now time.Time) bool {
	return d.IsValidAt(now)
}
