package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type DataSize int64

const (
	Byte     DataSize = 1
	Kilobyte          = 1024 * Byte
	Megabyte          = 1024 * Kilobyte
	Gigabyte          = 1024 * Megabyte
	Terabyte          = 1024 * Gigabyte
)

type sizeUnit struct {
	suffixes []string
	factor   DataSize
}

var sizeUnits = []sizeUnit{
	{suffixes: []string{"TB", "T"}, factor: Terabyte},
	{suffixes: []string{"GB", "G"}, factor: Gigabyte},
	{suffixes: []string{"MB", "M"}, factor: Megabyte},
	{suffixes: []string{"KB", "K"}, factor: Kilobyte},
	{suffixes: []string{"B"}, factor: Byte},
}

func (d DataSize) Bytes() int64 {
	return int64(d)
}

func (d DataSize) String() string {
	if d == 0 {
		return "0B"
	}

	for _, unit := range sizeUnits {
		if d%unit.factor == 0 {
			return fmt.Sprintf("%d%s", int64(d/unit.factor), unit.suffixes[0])
		}
	}

	return fmt.Sprintf("%dB", int64(d))
}

func ParseDataSize(value string) (DataSize, error) {
	text := strings.TrimSpace(value)
	if text == "" {
		return 0, errors.New("data size is empty")
	}

	digits := 0
	for digits < len(text) && (isDigit(text[digits]) || text[digits] == '.') {
		digits++
	}

	number := text[:digits]
	unit := strings.ToUpper(strings.TrimSpace(text[digits:]))

	if number == "" || strings.HasPrefix(number, ".") || strings.HasSuffix(number, ".") {
		return 0, fmt.Errorf("invalid data size %q", text)
	}

	factor := Byte

	if unit != "" {
		known, ok := factorFor(unit)
		if !ok {
			return 0, fmt.Errorf("unknown data size unit in %q", text)
		}

		factor = known
	}

	return scale(number, factor, text)
}

func factorFor(unit string) (DataSize, bool) {
	for _, candidate := range sizeUnits {
		for _, suffix := range candidate.suffixes {
			if unit == suffix {
				return candidate.factor, true
			}
		}
	}

	return 0, false
}

func scale(number string, factor DataSize, original string) (DataSize, error) {
	whole, fraction, hasFraction := strings.Cut(number, ".")

	parsedWhole, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid data size %q", original)
	}

	size := DataSize(parsedWhole) * factor
	if !hasFraction {
		return size, nil
	}

	parsedFraction, err := strconv.ParseInt(fraction, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid data size %q", original)
	}

	divisor := DataSize(1)
	for range len(fraction) {
		divisor *= 10
	}

	return size + DataSize(parsedFraction)*factor/divisor, nil
}

func isDigit(char byte) bool {
	return char >= '0' && char <= '9'
}

func (d DataSize) MarshalText() ([]byte, error) {
	return []byte(d.String()), nil
}

func (d *DataSize) UnmarshalText(text []byte) error {
	parsed, err := ParseDataSize(string(text))
	if err != nil {
		return err
	}

	*d = parsed

	return nil
}

func (d DataSize) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

func (d *DataSize) UnmarshalJSON(data []byte) error {
	var text string

	if err := json.Unmarshal(data, &text); err != nil {
		return fmt.Errorf("data size must be a string: %w", err)
	}

	return d.UnmarshalText([]byte(text))
}

func (d DataSize) MarshalYAML() (any, error) {
	return d.String(), nil
}

func (d *DataSize) UnmarshalYAML(node *yaml.Node) error {
	var text string

	if err := node.Decode(&text); err != nil {
		return fmt.Errorf("data size must be a string: %w", err)
	}

	return d.UnmarshalText([]byte(text))
}
