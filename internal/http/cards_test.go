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

func TestCreateCard_OK(t *testing.T) {
	st := newFakeStore()
	proc := &fakeProcessor{issueResult: processor.IssueResult{CardToken: "tok_secret_123", PANLast4: "4242", Expiry: "12/29"}}
	h := newHandlers(st, proc, &fakeLedger{})

	r := chiRequest(http.MethodPost, "/v1/cards", createCardRequest{AccountID: "acc-1"})
	w := httptest.NewRecorder()
	h.CreateCard(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "tok_secret_123") {
		t.Fatalf("response leaked the card token: %s", w.Body.String())
	}

	var got store.Card
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.AccountID != "acc-1" || got.PANLast4 != "4242" || got.Status != "active" {
		t.Fatalf("unexpected body: %+v", got)
	}
}

func TestCreateCard_CardnetError(t *testing.T) {
	st := newFakeStore()
	proc := &fakeProcessor{issueErr: errBoom}
	h := newHandlers(st, proc, &fakeLedger{})

	r := chiRequest(http.MethodPost, "/v1/cards", createCardRequest{AccountID: "acc-1"})
	w := httptest.NewRecorder()
	h.CreateCard(w, r)

	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d: %s", w.Code, w.Body.String())
	}
	if len(st.cards) != 0 {
		t.Fatalf("expected no card persisted when cardnet fails")
	}
}

// TestGetCard_DoesNotExposeToken is the spec's core PCI assertion:
// GET /v1/cards/{id} must never return card_token, no matter what's in the
// store.
func TestGetCard_DoesNotExposeToken(t *testing.T) {
	st := newFakeStore()
	st.cards["card-1"] = store.Card{
		ID: "card-1", AccountID: "acc-1", CardToken: "tok_should_never_leave", PANLast4: "4242", Expiry: "12/29", Status: "active",
	}
	h := newHandlers(st, &fakeProcessor{}, &fakeLedger{})

	r := chiRequest(http.MethodGet, "/v1/cards/card-1", nil, "id", "card-1")
	w := httptest.NewRecorder()
	h.GetCard(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "tok_should_never_leave") || strings.Contains(w.Body.String(), "card_token") {
		t.Fatalf("response exposed the card token: %s", w.Body.String())
	}

	var got store.Card
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.ID != "card-1" {
		t.Fatalf("unexpected body: %+v", got)
	}
}

func TestGetCard_NotFound(t *testing.T) {
	h := newHandlers(newFakeStore(), &fakeProcessor{}, &fakeLedger{})

	r := chiRequest(http.MethodGet, "/v1/cards/missing", nil, "id", "missing")
	w := httptest.NewRecorder()
	h.GetCard(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestListCardsByAccount_OK(t *testing.T) {
	st := newFakeStore()
	st.byAccount["acc-1"] = []store.Card{
		{ID: "card-1", AccountID: "acc-1", PANLast4: "4242", Status: "active"},
		{ID: "card-2", AccountID: "acc-1", PANLast4: "1111", Status: "active"},
	}
	h := newHandlers(st, &fakeProcessor{}, &fakeLedger{})

	r := chiRequest(http.MethodGet, "/v1/accounts/acc-1/cards", nil, "account_id", "acc-1")
	w := httptest.NewRecorder()
	h.ListCardsByAccount(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got []store.Card
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 cards, got %d", len(got))
	}
}

func TestBlockCard_OK(t *testing.T) {
	st := newFakeStore()
	st.cards["card-1"] = store.Card{ID: "card-1", AccountID: "acc-1", Status: "active"}
	h := newHandlers(st, &fakeProcessor{}, &fakeLedger{})

	r := chiRequest(http.MethodPost, "/v1/cards/card-1/block", nil, "id", "card-1")
	w := httptest.NewRecorder()
	h.BlockCard(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var got store.Card
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Status != "blocked" {
		t.Fatalf("expected status blocked, got %q", got.Status)
	}
}
