package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/demo-org-np-migration/cards-api/internal/ledger"
	"github.com/demo-org-np-migration/cards-api/internal/processor"
	"github.com/demo-org-np-migration/cards-api/internal/store"
)

// Handlers holds the dependencies every route needs. It's built once in
// router.go and its methods are wired up as chi handlers.
type Handlers struct {
	Store     store.Store
	Processor processor.Processor
	Ledger    ledger.Client
	Logger    *slog.Logger
}

type createCardRequest struct {
	AccountID string `json:"account_id"`
}

// CreateCard handles POST /v1/cards: issue a card with Cardnet, then
// persist it. card_token never leaves this handler in the response — the
// 201 body is the same store.Card GET /v1/cards/{id} returns, and
// store.Card.CardToken is json:"-".
func (h *Handlers) CreateCard(w http.ResponseWriter, r *http.Request) {
	var req createCardRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AccountID == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "account_id is required")
		return
	}

	issued, err := h.Processor.IssueCard(r.Context(), req.AccountID)
	if err != nil {
		h.Logger.Error("cardnet issue failed", "account_id", req.AccountID, "err", err)
		writeError(w, http.StatusBadGateway, "cardnet_unavailable", "could not issue the card")
		return
	}

	card, err := h.Store.CreateCard(r.Context(), store.Card{
		ID:        newID(),
		AccountID: req.AccountID,
		CardToken: issued.CardToken,
		PANLast4:  issued.PANLast4,
		Expiry:    issued.Expiry,
		Status:    "active",
	})
	if err != nil {
		h.Logger.Error("persist card failed", "account_id", req.AccountID, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save the card")
		return
	}

	writeJSON(w, http.StatusCreated, card)
}

// GetCard handles GET /v1/cards/{id}. store.Card.CardToken is json:"-", so
// there's no redaction to remember here — it simply never serializes.
func (h *Handlers) GetCard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	card, err := h.Store.GetCard(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "card_not_found", "no card with that id")
		return
	}
	if err != nil {
		h.Logger.Error("get card failed", "card_id", id, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load the card")
		return
	}

	writeJSON(w, http.StatusOK, card)
}

// ListCardsByAccount handles GET /v1/accounts/{account_id}/cards.
func (h *Handlers) ListCardsByAccount(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "account_id")

	cards, err := h.Store.ListCardsByAccount(r.Context(), accountID)
	if err != nil {
		h.Logger.Error("list cards by account failed", "account_id", accountID, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load the cards")
		return
	}

	writeJSON(w, http.StatusOK, cards)
}

// BlockCard handles POST /v1/cards/{id}/block.
func (h *Handlers) BlockCard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	card, err := h.Store.BlockCard(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "card_not_found", "no card with that id")
		return
	}
	if err != nil {
		h.Logger.Error("block card failed", "card_id", id, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not block the card")
		return
	}

	writeJSON(w, http.StatusOK, card)
}
