package clock

import "time"

// Clock provides time operations - allows mocking in tests
type Clock interface {
	Now() time.Time
}

// SystemClock uses real system time
type SystemClock struct{}

// NewSystemClock creates a new system clock
func NewSystemClock() Clock {
	return &SystemClock{}
}

// Now returns the current time
func (c *SystemClock) Now() time.Time {
	return time.Now()
}
