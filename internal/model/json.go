package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSONMap is a JSONB-backed string map.
type JSONMap map[string]any

// MarshalJSON implements json.Marshaler.
func (m JSONMap) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(map[string]any(m))
}

// UnmarshalJSON implements json.Unmarshaler.
func (m *JSONMap) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		*m = nil
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	*m = raw
	return nil
}

// Scan implements sql.Scanner for jsonb columns.
func (m *JSONMap) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*m = nil
		return nil
	case []byte:
		return m.UnmarshalJSON(v)
	case string:
		return m.UnmarshalJSON([]byte(v))
	default:
		return fmt.Errorf("cannot scan %T into model.JSONMap", src)
	}
}

// Value implements driver.Valuer.
func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(map[string]any(m))
}
