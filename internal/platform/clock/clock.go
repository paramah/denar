// Package clock supplies the current time.
package clock

import "time"

// System reads the operating system clock.
type System struct{}

// Now returns the current time in UTC, cut to the microsecond the database
// stores, so a value compares equal to itself after a round trip.
func (System) Now() time.Time {
	return time.Now().UTC().Truncate(time.Microsecond)
}
