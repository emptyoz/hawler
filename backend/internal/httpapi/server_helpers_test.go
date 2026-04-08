package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseOptionalDate(t *testing.T) {
	gotNil, err := parseOptionalDate("")
	if err != nil {
		t.Fatalf("parseOptionalDate(empty) error = %v", err)
	}
	if gotNil != nil {
		t.Fatalf("parseOptionalDate(empty) = %v, want nil", gotNil)
	}

	got, err := parseOptionalDate("2026-03-15")
	if err != nil {
		t.Fatalf("parseOptionalDate(valid) error = %v", err)
	}
	if got == nil {
		t.Fatalf("parseOptionalDate(valid) returned nil date")
	}
	if got.Format("2006-01-02") != "2026-03-15" {
		t.Fatalf("parseOptionalDate(valid) = %s, want %s", got.Format("2006-01-02"), "2026-03-15")
	}

	if _, err := parseOptionalDate("15-03-2026"); err == nil {
		t.Fatalf("parseOptionalDate(invalid) expected error, got nil")
	}
}

func TestDecodeJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	t.Run("valid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"ok"}`))
		var got payload
		if err := decodeJSON(req, &got); err != nil {
			t.Fatalf("decodeJSON(valid) error = %v", err)
		}
		if got.Name != "ok" {
			t.Fatalf("decodeJSON(valid) name = %q, want %q", got.Name, "ok")
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"ok","extra":1}`))
		var got payload
		if err := decodeJSON(req, &got); err == nil {
			t.Fatalf("decodeJSON(unknown field) expected error, got nil")
		}
	})

	t.Run("multiple json values", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"ok"}{"name":"second"}`))
		var got payload
		if err := decodeJSON(req, &got); err == nil {
			t.Fatalf("decodeJSON(multiple values) expected error, got nil")
		}
	})
}

func TestNormalizeColumnKind(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "To Do", want: "to_do"},
		{in: " in-progress ", want: "in_progress"},
		{in: "QA", want: "qa"},
		{in: "", want: ""},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			got := normalizeColumnKind(tc.in)
			if got != tc.want {
				t.Fatalf("normalizeColumnKind(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseOptionalDateTrimsSpaces(t *testing.T) {
	got, err := parseOptionalDate(" 2026-03-15 ")
	if err != nil {
		t.Fatalf("parseOptionalDate(trimmed) error = %v", err)
	}
	if got == nil {
		t.Fatalf("parseOptionalDate(trimmed) returned nil date")
	}
	if !got.Equal(time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("parseOptionalDate(trimmed) = %v, want 2026-03-15 UTC", got)
	}
}
