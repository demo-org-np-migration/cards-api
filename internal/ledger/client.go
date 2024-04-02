// Package ledger records the debit side of an approved card authorization
// against ledger-core, using cauri-go-kit/httpx with a client-credentials
// TokenProvider for the cards-api → ledger-core service call.
package ledger

import (
	"context"
	"fmt"

	"github.com/demo-org-np-migration/cauri-go-kit/httpx"
)

// Client is what cards-api's handlers need from ledger-core. Tests use a
// fake; LedgerClient is the real thing.
type Client interface {
	RecordDebit(ctx context.Context, accountID, amount, reference string) error
}

// LedgerClient is the real Client, calling ledger-core's
// POST /v1/ledger/entries.
type LedgerClient struct {
	http *httpx.Client
}

// NewLedgerClient builds a LedgerClient against baseURL (LEDGER_CORE_URL),
// authenticating every request with the token tokenProvider hands out.
func NewLedgerClient(baseURL string, tokenProvider httpx.TokenProvider) *LedgerClient {
	return &LedgerClient{
		http: httpx.NewClient(baseURL, httpx.WithTokenProvider(tokenProvider)),
	}
}

// RecordDebit posts a type=debit entry for accountID with reference
// "card-auth:<auth_id>" (built by the caller), per las convenciones internas de API.
// ledger_entries has no currency column (las convenciones internas de API) — the account's
// own currency is authoritative, so only amount, type and reference travel.
func (c *LedgerClient) RecordDebit(ctx context.Context, accountID, amount, reference string) error {
	req := struct {
		AccountID string `json:"account_id"`
		Amount    string `json:"amount"`
		Type      string `json:"type"`
		Reference string `json:"reference"`
	}{AccountID: accountID, Amount: amount, Type: "debit", Reference: reference}

	if err := c.http.PostJSON(ctx, "/v1/ledger/entries", req, nil); err != nil {
		return fmt.Errorf("ledger: record debit: %w", err)
	}
	return nil
}
