package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Layouts used for the JSON representation of dates and clock times.
const (
	DateLayout         = "2006-01-02"
	ClockLayout        = "15:04"
	ClockLayoutSeconds = "15:04:05"
)

// Date is a calendar date without time zone, serialized as "YYYY-MM-DD".
type Date struct {
	time.Time
}

// NewDate normalises t to midnight UTC.
func NewDate(t time.Time) Date {
	return Date{time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)}
}

// Today returns the current date in loc.
func Today(loc *time.Location) Date {
	return NewDate(time.Now().In(loc))
}

// ParseDate parses a "YYYY-MM-DD" string.
func ParseDate(s string) (Date, error) {
	t, err := time.Parse(DateLayout, strings.TrimSpace(s))
	if err != nil {
		return Date{}, fmt.Errorf("invalid date %q: expected format YYYY-MM-DD", s)
	}
	return Date{t}, nil
}

// MarshalJSON implements json.Marshaler.
func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Format(DateLayout))
}

// UnmarshalJSON implements json.Unmarshaler.
func (d *Date) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("date must be a string in YYYY-MM-DD format")
	}
	parsed, err := ParseDate(s)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// Scan implements sql.Scanner for PostgreSQL `date` values.
func (d *Date) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*d = Date{}
		return nil
	case time.Time:
		*d = NewDate(v)
		return nil
	case string:
		parsed, err := ParseDate(v)
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	case []byte:
		parsed, err := ParseDate(string(v))
		if err != nil {
			return err
		}
		*d = parsed
		return nil
	default:
		return fmt.Errorf("cannot scan %T into model.Date", src)
	}
}

// Value implements driver.Valuer.
func (d Date) Value() (driver.Value, error) {
	if d.Time.IsZero() {
		return nil, nil
	}
	return d.Format(DateLayout), nil
}

// String returns the date in YYYY-MM-DD format.
func (d Date) String() string { return d.Format(DateLayout) }

// IsZero reports whether the date is unset.
func (d Date) IsZero() bool { return d.Time.IsZero() }

// DayType classifies the date as weekend or weekday.
func (d Date) DayType() DayType {
	if wd := d.Time.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return DayTypeWeekend
	}
	return DayTypeWeekday
}

// AtIn combines the date with a clock time in the given location.
func (d Date) AtIn(c Clock, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	return time.Date(d.Year(), d.Month(), d.Day(), c.Hour(), c.Minute(), 0, 0, loc)
}

// AddDays returns the date shifted by n days.
func (d Date) AddDays(n int) Date { return NewDate(d.Time.AddDate(0, 0, n)) }

// Clock is a wall-clock time of day, serialized as "HH:MM".
type Clock struct {
	time.Time
}

// NewClock builds a Clock from hour and minute.
func NewClock(hour, minute int) Clock {
	return Clock{time.Date(0, 1, 1, hour, minute, 0, 0, time.UTC)}
}

// ParseClock parses "HH:MM" or "HH:MM:SS".
func ParseClock(s string) (Clock, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{ClockLayout, ClockLayoutSeconds} {
		if t, err := time.Parse(layout, s); err == nil {
			return NewClock(t.Hour(), t.Minute()), nil
		}
	}
	return Clock{}, fmt.Errorf("invalid time %q: expected format HH:MM", s)
}

// MarshalJSON implements json.Marshaler.
func (c Clock) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.Format(ClockLayout))
}

// UnmarshalJSON implements json.Unmarshaler.
func (c *Clock) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("time must be a string in HH:MM format")
	}
	parsed, err := ParseClock(s)
	if err != nil {
		return err
	}
	*c = parsed
	return nil
}

// Scan implements sql.Scanner for PostgreSQL `time` values.
func (c *Clock) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*c = Clock{}
		return nil
	case time.Time:
		*c = NewClock(v.Hour(), v.Minute())
		return nil
	case string:
		parsed, err := ParseClock(v)
		if err != nil {
			return err
		}
		*c = parsed
		return nil
	case []byte:
		parsed, err := ParseClock(string(v))
		if err != nil {
			return err
		}
		*c = parsed
		return nil
	default:
		return fmt.Errorf("cannot scan %T into model.Clock", src)
	}
}

// Value implements driver.Valuer.
func (c Clock) Value() (driver.Value, error) {
	if c.Time.IsZero() {
		return nil, nil
	}
	return c.Format(ClockLayoutSeconds), nil
}

// String returns the clock time in HH:MM format.
func (c Clock) String() string { return c.Format(ClockLayout) }

// IsZero reports whether the clock time is unset.
func (c Clock) IsZero() bool { return c.Time.IsZero() }

// Minutes returns minutes since midnight.
func (c Clock) Minutes() int { return c.Hour()*60 + c.Minute() }

// Before reports whether c is earlier in the day than other.
func (c Clock) Before(other Clock) bool { return c.Minutes() < other.Minutes() }

// After reports whether c is later in the day than other.
func (c Clock) After(other Clock) bool { return c.Minutes() > other.Minutes() }

// Equal reports whether both clock times are the same minute of day.
func (c Clock) Equal(other Clock) bool { return c.Minutes() == other.Minutes() }

// AddMinutes returns the clock time shifted by n minutes.
func (c Clock) AddMinutes(n int) Clock { return NewClock((c.Minutes()+n)/60%24, (c.Minutes()+n)%60) }

// MinutesBetween returns other - c in minutes.
func (c Clock) MinutesBetween(other Clock) int { return other.Minutes() - c.Minutes() }
