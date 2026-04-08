package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"hawler/backend/internal/store"
)

func decodeErrorBody(t *testing.T, body string) map[string]string {
	t.Helper()
	out := map[string]string{}
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("json.Unmarshal() error = %v, body = %q", err, body)
	}
	return out
}

func TestEmailAndRoleValidators(t *testing.T) {
	t.Run("isValidEmail", func(t *testing.T) {
		cases := []struct {
			in   string
			want bool
		}{
			{in: "user@example.com", want: true},
			{in: "u@x", want: true},
			{in: "userexample.com", want: false},
			{in: "@example.com", want: false},
			{in: "user@", want: false},
			{in: "user@@example.com", want: false},
		}

		for _, tc := range cases {
			tc := tc
			t.Run(tc.in, func(t *testing.T) {
				t.Parallel()
				if got := isValidEmail(tc.in); got != tc.want {
					t.Fatalf("isValidEmail(%q) = %v, want %v", tc.in, got, tc.want)
				}
			})
		}
	})

	t.Run("isValidWorkspaceRole", func(t *testing.T) {
		if !isValidWorkspaceRole(store.RoleOwner) {
			t.Fatalf("owner should be valid workspace role")
		}
		if !isValidWorkspaceRole(store.RoleMentor) {
			t.Fatalf("mentor should be valid workspace role")
		}
		if !isValidWorkspaceRole(store.RoleStudent) {
			t.Fatalf("student should be valid workspace role")
		}
		if isValidWorkspaceRole("admin") {
			t.Fatalf("admin should not be valid workspace role")
		}
	})

	t.Run("hasAllowedRole", func(t *testing.T) {
		if !hasAllowedRole("owner", nil) {
			t.Fatalf("empty allowed list should allow any role")
		}
		if !hasAllowedRole("owner", []string{"owner", "mentor"}) {
			t.Fatalf("owner should be allowed")
		}
		if hasAllowedRole("student", []string{"owner", "mentor"}) {
			t.Fatalf("student should not be allowed")
		}
	})
}

func TestDBErrorClassifiers(t *testing.T) {
	if !isFKError(sql.ErrNoRows) {
		t.Fatalf("isFKError(sql.ErrNoRows) = false, want true")
	}
	if !isFKError(errors.New("insert violates FOREIGN KEY constraint")) {
		t.Fatalf("isFKError(foreign key msg) = false, want true")
	}
	if isFKError(errors.New("some other db error")) {
		t.Fatalf("isFKError(other msg) = true, want false")
	}

	if !isUniqueViolation(errors.New("duplicate key value violates unique constraint")) {
		t.Fatalf("isUniqueViolation(duplicate key) = false, want true")
	}
	if !isUniqueViolation(errors.New("UNIQUE constraint failed")) {
		t.Fatalf("isUniqueViolation(unique msg) = false, want true")
	}
	if isUniqueViolation(errors.New("timeout")) {
		t.Fatalf("isUniqueViolation(timeout) = true, want false")
	}
}

func TestWriteErrorAndAccessErrorHelpers(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, http.StatusTeapot, "boom")
	if rec.Code != http.StatusTeapot {
		t.Fatalf("writeError status = %d, want %d", rec.Code, http.StatusTeapot)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", rec.Header().Get("Content-Type"))
	}
	payload := decodeErrorBody(t, rec.Body.String())
	if payload["error"] != "boom" {
		t.Fatalf("error payload = %q, want %q", payload["error"], "boom")
	}

	srv := &Server{}

	rec = httptest.NewRecorder()
	srv.writeAccessError(rec, errForbidden)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("writeAccessError(forbidden) status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	rec = httptest.NewRecorder()
	srv.writeAccessError(rec, sql.ErrNoRows)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("writeAccessError(sql.ErrNoRows) status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	rec = httptest.NewRecorder()
	srv.writeTaskAccessError(rec, errTaskIDRequired)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("writeTaskAccessError(task id required) status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec = httptest.NewRecorder()
	srv.writeTaskAccessError(rec, errForbidden)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("writeTaskAccessError(forbidden) status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestRequireAuthMiddleware(t *testing.T) {
	srv := &Server{
		auth: NewAuthManager("test-secret", time.Hour),
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := currentUserFromContext(r.Context())
		if !ok {
			t.Fatalf("currentUserFromContext() ok = false, want true")
		}
		w.Header().Set("X-User-ID", user.ID)
		w.WriteHeader(http.StatusNoContent)
	})
	handler := srv.requireAuth(next)

	t.Run("missing authorization", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("invalid authorization format", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
		req.Header.Set("Authorization", "Token abc")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
		req.Header.Set("Authorization", "Bearer bad-token")
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("valid token", func(t *testing.T) {
		token, err := srv.auth.GenerateToken("user-42", "user42@example.com")
		if err != nil {
			t.Fatalf("GenerateToken() error = %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
		}
		if rec.Header().Get("X-User-ID") != "user-42" {
			t.Fatalf("X-User-ID = %q, want %q", rec.Header().Get("X-User-ID"), "user-42")
		}
	})
}

func TestDevCORSMiddleware(t *testing.T) {
	nextCalled := false
	handler := devCORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("preflight options", func(t *testing.T) {
		nextCalled = false
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/tasks", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
		}
		if nextCalled {
			t.Fatalf("next handler should not be called for OPTIONS")
		}
		if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Fatalf("allow-origin = %q, want *", rec.Header().Get("Access-Control-Allow-Origin"))
		}
	})

	t.Run("non options", func(t *testing.T) {
		nextCalled = false
		req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
		}
		if !nextCalled {
			t.Fatalf("next handler should be called for GET")
		}
		if rec.Header().Get("Access-Control-Allow-Headers") == "" {
			t.Fatalf("expected CORS headers to be set")
		}
	})
}
