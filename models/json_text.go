package models

import (
	"database/sql/driver"
	"fmt"
)

// JSONText is a JSON document stored as text. It is written to the database as
// a string so the server does not treat it as a binary string. Columns that
// must preserve canonical JSON bytes use a longtext column type.
type JSONText []byte

func (JSONText) GormDataType() string {
	return "json"
}

func (j JSONText) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}

func (j *JSONText) Scan(val interface{}) error {
	switch v := val.(type) {
	case nil:
		*j = nil
	case []byte:
		*j = append((*j)[:0], v...)
	case string:
		*j = JSONText(v)
	default:
		return fmt.Errorf("JSONText: unsupported scan type %T", val)
	}
	return nil
}

func (j JSONText) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return j, nil
}
