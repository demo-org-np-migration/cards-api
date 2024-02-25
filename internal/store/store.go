// Package store is cards-api's data layer against the `cards` database it
// owns (las convenciones internas de API): the cards themselves (token + last4, never the
// PAN) and the authorizations run against them.
package store

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned by any lookup that finds nothing, so handlers can
// map it to a 404 without inspecting driver-specific errors.
var ErrNotFound = errors.New("store: not found")

// Card mirrors a row of `cards`. CardToken is tagged json:"-" so it can
// never leak through an HTTP response no matter which handler serializes
// a Card — GET /v1/cards/{id} must never return it (PCI scope, see
// docs/PCI.md), and centralizing the redaction here means no handler has
// to remember to strip it by hand.
type Card struct {
	ID        string    `json:"id"`
	AccountID string    `json:"account_id"`
	CardToken string    `json:"-"`
	PANLast4  string    `json:"pan_last4"`
	Expiry    string    `json:"expiry"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Authorization mirrors a row of `authorizations`. Persisted whether
// Cardnet approved or declined, so the merchant/customer-facing history is
// complete either way.
type Authorization struct {
	ID           string    `json:"id"`
	CardID       string    `json:"card_id"`
	Amount       string    `json:"amount"`
	Currency     string    `json:"currency"`
	MerchantName string    `json:"merchant_name"`
	AuthCode     string    `json:"auth_code,omitempty"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

// Store is everything the HTTP layer needs from the data layer. Handlers
// are tested against a fake implementation; Postgres is the real one.
type Store interface {
	// CreateCard persists a newly issued card and returns it.
	CreateCard(ctx context.Context, card Card) (*Card, error)

	// GetCard returns the card with that id. ErrNotFound if there isn't one.
	GetCard(ctx context.Context, id string) (*Card, error)

	// ListCardsByAccount returns every card belonging to accountID, in no
	// particular order. An empty slice, not an error, if there are none.
	ListCardsByAccount(ctx context.Context, accountID string) ([]Card, error)

	// BlockCard sets status='blocked' on the card and returns the updated
	// row. ErrNotFound if no card has that id.
	BlockCard(ctx context.Context, id string) (*Card, error)

	// CreateAuthorization persists the outcome of an authorization attempt
	// — approved or declined — and returns it.
	CreateAuthorization(ctx context.Context, auth Authorization) (*Authorization, error)
}
