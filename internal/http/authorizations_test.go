package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/demo-org-np-migration/cards-api/internal/processor"
	"github.com/demo-org-np-migration/cards-api/internal/store"
)

func TestCreateAuthorization_Approved_RecordsDebit(t *testing.T) {
	st := newFakeStore()
	st.cards["card-1"] = store.Card{ID: "card-1", AccountID: "acc-1", CardToken: "tok_1", Status: "active"}
	proc := &fakeProcessor{authorizeResult: processor.AuthorizeResult{AuthCode: "auth_123", Status: "approved"}}
	ledgerClient := &fakeLedger{}
	h := newHandlers(st, proc, ledgerClient)

	r := chiRequest(http.MethodPost, "/v1/authorizations", createAuthorizationRequest{
		CardID: "card-1", Amount: "1250.50", Currency: "ARS", MerchantName: "Almacén Bruno",
	})
	w := httptest.NewRecorder()
	h.CreateAuthorization(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var got store.Authorization
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Status != "approved" {
		t.Fatalf("expected status approved, got %q", got.Status)
	}

	if len(ledgerClient.debits) != 1 {
		t.Fatalf("expected exactly one ledger debit, got %d", len(ledgerClient.debits))
	}
	debit := ledgerClient.debits[0]
	if debit.accountID != "acc-1" || debit.amount != "1250.50" {
		t.Fatalf("unexpected debit: %+v", debit)
	}
	if !strings.HasPrefix(debit.reference, "card-auth:") {
		t.Fatalf("expected reference to start with card-auth:, got %q", debit.reference)
	}
	if !strings.Contains(debit.reference, got.ID) {
		t.Fatalf("expected reference to carry the authorization id %q, got %q", got.ID, debit.reference)
	}
}

func TestCreateAuthorization_Declined_DoesNotTouchLedger(t *testing.T) {
	st := newFakeStore()
	st.cards["card-1"] = store.Card{ID: "card-1", AccountID: "acc-1", CardToken: "tok_1", Status: "active"}
	proc := &fakeProcessor{authorizeResult: processor.AuthorizeResult{Status: "declined"}}
	ledgerClient := &fakeLedger{}
	h := newHandlers(st, proc, ledgerClient)

	r := chiRequest(http.MethodPost, "/v1/authorizations", createAuthorizationRequest{
		CardID: "card-1", Amount: "999999.00", Currency: "ARS", MerchantName: "Almacén Bruno",
	})
	w := httptest.NewRecorder()
	h.CreateAuthorization(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var got store.Authorization
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Status != "declined" {
		t.Fatalf("expected status declined, got %q", got.Status)
	}

	if len(ledgerClient.debits) != 0 {
		t.Fatalf("expected no ledger debit for a declined authorization, got %d", len(ledgerClient.debits))
	}
	if len(st.authorizations) != 1 {
		t.Fatalf("expected the declined attempt to still be persisted")
	}
}

func TestCreateAuthorization_CardNotFound(t *testing.T) {
	h := newHandlers(newFakeStore(), &fakeProcessor{}, &fakeLedger{})

	r := chiRequest(http.MethodPost, "/v1/authorizations", createAuthorizationRequest{
		CardID: "missing", Amount: "10.00", Currency: "ARS", MerchantName: "Café Cauri",
	})
	w := httptest.NewRecorder()
	h.CreateAuthorization(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}
