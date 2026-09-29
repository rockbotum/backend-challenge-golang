package config

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDuration_UnmarshalText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  time.Duration
	}{
		{name: "seconds", input: "2s", want: 2 * time.Second},
		{name: "compound", input: "1m30s", want: 90 * time.Second},
		{name: "milliseconds", input: "250ms", want: 250 * time.Millisecond},
		{name: "zero", input: "0s", want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var duration Duration

			if err := duration.UnmarshalText([]byte(test.input)); err != nil {
				t.Fatalf("UnmarshalText(%q) returned %v", test.input, err)
			}

			if got := duration.Duration(); got != test.want {
				t.Errorf("UnmarshalText(%q) = %s, want %s", test.input, got, test.want)
			}
		})
	}
}

func TestDuration_UnmarshalText_Invalid(t *testing.T) {
	t.Parallel()

	var duration Duration

	err := duration.UnmarshalText([]byte("soon"))
	if err == nil {
		t.Fatal("UnmarshalText(\"soon\") returned nil, want error")
	}
}

func TestDuration_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	type holder struct {
		Value Duration `json:"value"`
	}

	payload := []byte(`{"value":"3s"}`)

	var decoded holder
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("Unmarshal returned %v", err)
	}

	if decoded.Value.Duration() != 3*time.Second {
		t.Errorf("decoded duration = %s, want 3s", decoded.Value.Duration())
	}

	encoded, err := json.Marshal(decoded)
	if err != nil {
		t.Fatalf("Marshal returned %v", err)
	}

	if string(encoded) != `{"value":"3s"}` {
		t.Errorf("Marshal = %s, want {\"value\":\"3s\"}", encoded)
	}
}

func TestDuration_UnmarshalJSON_RejectsNumber(t *testing.T) {
	t.Parallel()

	var duration Duration

	if err := json.Unmarshal([]byte(`30`), &duration); err == nil {
		t.Fatal("Unmarshal(30) returned nil, want error")
	}
}

func TestDuration_String(t *testing.T) {
	t.Parallel()

	if got := Duration(90 * time.Second).String(); got != "1m30s" {
		t.Errorf("String() = %q, want %q", got, "1m30s")
	}
}
