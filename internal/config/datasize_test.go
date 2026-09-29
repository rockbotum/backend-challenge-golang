package config

import (
	"encoding/json"
	"testing"
)

func TestParseDataSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  DataSize
	}{
		{name: "bytes", input: "512B", want: 512},
		{name: "no unit is bytes", input: "2048", want: 2048},
		{name: "kilobytes", input: "512KB", want: 512 * Kilobyte},
		{name: "megabytes", input: "10MB", want: 10 * Megabyte},
		{name: "gigabytes", input: "1GB", want: Gigabyte},
		{name: "terabytes", input: "2TB", want: 2 * Terabyte},
		{name: "short unit", input: "4M", want: 4 * Megabyte},
		{name: "lower case", input: "1gb", want: Gigabyte},
		{name: "surrounding space", input: " 10MB ", want: 10 * Megabyte},
		{name: "fraction", input: "1.5MB", want: Megabyte + 512*Kilobyte},
		{name: "zero", input: "0MB", want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseDataSize(test.input)
			if err != nil {
				t.Fatalf("ParseDataSize(%q) returned %v", test.input, err)
			}

			if got != test.want {
				t.Errorf("ParseDataSize(%q) = %d, want %d", test.input, int64(got), int64(test.want))
			}
		})
	}
}

func TestParseDataSize_Invalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "unknown unit", input: "10PB"},
		{name: "missing number", input: "MB"},
		{name: "not a number", input: "tenMB"},
		{name: "missing leading digit", input: ".5MB"},
		{name: "missing fraction digits", input: "1.MB"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if _, err := ParseDataSize(test.input); err == nil {
				t.Errorf("ParseDataSize(%q) returned nil, want error", test.input)
			}
		})
	}
}

func TestDataSize_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input DataSize
		want  string
	}{
		{name: "zero", input: 0, want: "0B"},
		{name: "exact megabytes", input: 10 * Megabyte, want: "10MB"},
		{name: "not divisible", input: 1536, want: "1536B"},
		{name: "gigabytes", input: 3 * Gigabyte, want: "3GB"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := test.input.String(); got != test.want {
				t.Errorf("String() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDataSize_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	type holder struct {
		Size DataSize `json:"size"`
	}

	var decoded holder
	if err := json.Unmarshal([]byte(`{"size":"10MB"}`), &decoded); err != nil {
		t.Fatalf("Unmarshal returned %v", err)
	}

	if decoded.Size != 10*Megabyte {
		t.Errorf("decoded size = %d, want %d", decoded.Size.Bytes(), int64(10*Megabyte))
	}

	encoded, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("Marshal returned %v", err)
	}

	if string(encoded) != `{"size":"10MB"}` {
		t.Errorf("Marshal = %s, want {\"size\":\"10MB\"}", encoded)
	}
}

func TestDataSize_UnmarshalJSON_RejectsNumber(t *testing.T) {
	t.Parallel()

	var size DataSize

	if err := json.Unmarshal([]byte(`10485760`), &size); err == nil {
		t.Fatal("Unmarshal(10485760) returned nil, want error")
	}
}

func TestDataSize_Bytes(t *testing.T) {
	t.Parallel()

	if got := (2 * Megabyte).Bytes(); got != 2097152 {
		t.Errorf("Bytes() = %d, want 2097152", got)
	}
}
