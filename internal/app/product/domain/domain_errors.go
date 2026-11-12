package domain

import "errors"

// Domain errors are sentinel values for clear error handling
var (
	ErrProductNotFound        = errors.New("product not found")
	ErrProductNotActive       = errors.New("product is not active")
	ErrProductAlreadyActive   = errors.New("product is already active")
	ErrProductAlreadyArchived = errors.New("product is already archived")
	ErrInvalidProductName     = errors.New("product name cannot be empty")
	ErrInvalidCategory        = errors.New("category cannot be empty")
	ErrInvalidPrice           = errors.New("price must be positive")
	ErrInvalidDiscountPeriod  = errors.New("discount period is invalid: it must end after it starts and include the current time")
	ErrDiscountPercentInvalid = errors.New("discount percentage must be between 1 and 100")
	ErrDiscountAlreadyActive  = errors.New("product already has an active discount")
	ErrNoActiveDiscount       = errors.New("product has no active discount")
)
