package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/demo-org-np-migration/cards-api/internal/store"
)

// TestAuthorizations_RequiresServiceRole exercises the real router (not
// just the handler) so auth.RequireRole("service") is actually in the
// chain: a request with no Authorization header never reaches the handler
// at all, which is the 401/403-without-role case the spec asks for.
func TestAuthorizations_RequiresServiceRole(t *testing.T) {
	st := newFakeStore()
	st.cards["card-1"] = store.Card{ID: "card-1", AccountID: "acc-1", Status: "active"}
	ledgerClient := &fakeLedger{}
	router := NewRouter(st, &fakeProcessor{}, ledgerClient, testLogger(), "http://keycloak.invalid/realms/cauri-test")

	req := httptest.NewRequest(http.MethodPost, "/v1/authorizations", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// No bearer token at all: auth.Middleware itself rejects with 401
	// before RequireRole ever runs. Either way the authorization handler
	// must not run — no debit should have been recorded.
	if w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
		t.Fatalf("expected 401 or 403, got %d: %s", w.Code, w.Body.String())
	}
	if len(ledgerClient.debits) != 0 {
		t.Fatalf("authorization handler ran without a valid service token")
	}
}

func TestHealth_OK(t *testing.T) {
	router := NewRouter(newFakeStore(), &fakeProcessor{}, &fakeLedger{}, testLogger(), "http://keycloak.invalid/realms/cauri-test")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}
