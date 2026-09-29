package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// DataSize хранит размер данных в байтах.
type DataSize int64

const (
	Byte     DataSize = 1
	Kilobyte          = 1024 * Byte
	Megabyte          = 1024 * Kilobyte
	Gigabyte          = 1024 * Megabyte
	Terabyte          = 1024 * Gigabyte
)

func Bytes(n int64) DataSize {
	return DataSize(n)
}

func KB(n int64) DataSize {
	return DataSize(n) * Kilobyte
}

func MB(n int64) DataSize {
	return DataSize(n) * Megabyte
}

func GB(n int64) DataSize {
	return DataSize(n) * Gigabyte
}

func (d DataSize) Bytes() int64 {
	return int64(d)
}

func (d DataSize) KB() float64 {
	return float64(d) / float64(Kilobyte)
}

func (d DataSize) MB() float64 {
	return float64(d) / float64(Megabyte)
}

func (d DataSize) GB() float64 {
	return float64(d) / float64(Gigabyte)
}

func (d DataSize) String() string {
	units := []struct {
		divider DataSize
		name    string
	}{
		{Terabyte, "TB"},
		{Gigabyte, "GB"},
		{Megabyte, "MB"},
		{Kilobyte, "KB"},
		{Byte, "B"},
	}

	for _, unit := range units {
		if d%unit.divider == 0 {
			return fmt.Sprintf("%d%s", d/unit.divider, unit.name)
		}
	}

	return fmt.Sprintf("%dB", d)
}

func ParseDataSize(value string) (DataSize, error) {
	text := strings.ToUpper(strings.TrimSpace(value))

	units := []struct {
		name   string
		factor DataSize
	}{
		{"TB", Terabyte},
		{"GB", Gigabyte},
		{"MB", Megabyte},
		{"KB", Kilobyte},
		{"B", Byte},
	}

	for _, unit := range units {
		if strings.HasSuffix(text, unit.name) {
			number := strings.TrimSpace(
				strings.TrimSuffix(text, unit.name),
			)

			value, err := strconv.ParseFloat(number, 64)
			if err != nil || value < 0 {
				return 0, fmt.Errorf(
					"invalid data size %q",
					text,
				)
			}

			return DataSize(value * float64(unit.factor)), nil
		}
	}

	// Если суффикс не указан, значение считается количеством байт.
	valueInt, err := strconv.ParseInt(text, 10, 64)
	if err != nil || valueInt < 0 {
		return 0, fmt.Errorf(
			"invalid data size %q",
			text,
		)
	}

	return DataSize(valueInt), nil
}

// Поддержка YAML и других текстовых конфигураций.
func (d *DataSize) UnmarshalText(text []byte) error {
	value, err := ParseDataSize(string(text))
	if err != nil {
		return err
	}

	*d = value
	return nil
}

// JSON будет выглядеть как строка: "10MB".
func (d DataSize) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *DataSize) UnmarshalJSON(data []byte) error {
	var value string

	if err := json.Unmarshal(data, &value); err != nil {
		return fmt.Errorf(
			"data size must be a string: %w",
			err,
		)
	}

	parsed, err := ParseDataSize(value)
	if err != nil {
		return err
	}

	*d = parsed
	return nil
}
