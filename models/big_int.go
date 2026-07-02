package models

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"math/big"
	"strings"
)

type BigInt struct {
	big.Int
}

func (i *BigInt) Scan(val interface{}) error {
	var intStr string
	switch v := val.(type) {
	case string:
		intStr = v
	case []byte:
		intStr = string(v)
	case int:
		intStr = fmt.Sprint(v)
	case int8:
		intStr = fmt.Sprint(v)
	case int16:
		intStr = fmt.Sprint(v)
	case int32:
		intStr = fmt.Sprint(v)
	case int64:
		intStr = fmt.Sprint(v)
	case uint:
		intStr = fmt.Sprint(v)
	case uint8:
		intStr = fmt.Sprint(v)
	case uint16:
		intStr = fmt.Sprint(v)
	case uint32:
		intStr = fmt.Sprint(v)
	case uint64:
		intStr = fmt.Sprint(v)
	case nil:
		return nil
	default:
		return errors.New(fmt.Sprint("Unable to convert BigInt value to string: ", val))
	}

	_, success := i.SetString(intStr, 10)
	if !success {
		return errors.New("Unable to parse BigInt string: " + intStr)
	}
	return nil
}

func (i BigInt) Value() (driver.Value, error) {
	return i.String(), nil
}

func (i BigInt) MarshalText() ([]byte, error) {
	s := i.String()
	return fmt.Appendf(nil, "\"%s\"", s), nil
}

func (i *BigInt) UnmarshalText(data []byte) error {
	s := string(data)
	s = strings.Trim(s, "\"")

	var z big.Int
	_, ok := z.SetString(s, 10)
	if !ok {
		return fmt.Errorf("not a valid big integer: %s", s)
	}
	i.Int = z
	return nil
}

func (i BigInt) MarshalJSON() ([]byte, error) {
	return i.MarshalText()
}

func (i *BigInt) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	return i.UnmarshalText(data)
}
