package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/demo-org-np-migration/cards-api/internal/store"
)

type createAuthorizationRequest struct {
	CardID       string `json:"card_id"`
	Amount       string `json:"amount"`
	Currency     string `json:"currency"`
	MerchantName string `json:"merchant_name"`
}

// CreateAuthorization handles POST /v1/authorizations. The router only
// reaches this handler for requests that already passed
// auth.RequireRole("service").
//
// Flow (las convenciones internas de API "cards-api"): look up the card being charged,
// authorize with Cardnet, and — only if Cardnet approves — record a debit
// against ledger-core with reference "card-auth:<auth_id>". Either way the
// attempt is persisted, approved or declined, so authorization history is
// complete.
func (h *Handlers) CreateAuthorization(w http.ResponseWriter, r *http.Request) {
	var req createAuthorizationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil ||
		req.CardID == "" || req.Amount == "" || req.Currency == "" || req.MerchantName == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "card_id, amount, currency and merchant_name are required")
		return
	}

	card, err := h.Store.GetCard(r.Context(), req.CardID)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "card_not_found", "no card with that id")
		return
	}
	if err != nil {
		h.Logger.Error("get card for authorization failed", "card_id", req.CardID, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not load the card")
		return
	}

	result, err := h.Processor.Authorize(r.Context(), card.CardToken, req.Amount, req.Currency, req.MerchantName)
	if err != nil {
		h.Logger.Error("cardnet authorize failed", "card_id", req.CardID, "err", err)
		writeError(w, http.StatusBadGateway, "cardnet_unavailable", "could not authorize with the processor")
		return
	}

	authID := newID()

	if result.Status == "approved" {
		reference := "card-auth:" + authID
		if err := h.Ledger.RecordDebit(r.Context(), card.AccountID, req.Amount, reference); err != nil {
			h.Logger.Error("ledger debit failed", "authorization_id", authID, "card_id", req.CardID, "err", err)
			writeError(w, http.StatusBadGateway, "ledger_unavailable", "authorized but could not record the debit")
			return
		}
	}

	auth, err := h.Store.CreateAuthorization(r.Context(), store.Authorization{
		ID:           authID,
		CardID:       req.CardID,
		Amount:       req.Amount,
		Currency:     req.Currency,
		MerchantName: req.MerchantName,
		AuthCode:     result.AuthCode,
		Status:       result.Status,
	})
	if err != nil {
		h.Logger.Error("persist authorization failed", "authorization_id", authID, "err", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "could not save the authorization")
		return
	}

	writeJSON(w, http.StatusCreated, auth)
}
