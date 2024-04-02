// Package processor talks to Cardnet, the card processor vendor
// (las convenciones internas de API): issuing cards and authorizing purchases against them.
//
// It doesn't use cauri-go-kit/httpx — that client's auth is a bearer token
// from a TokenProvider, and Cardnet authenticates with a static
// X-Cardnet-Key header instead, so this is a small client of its own with
// the same shape (JSON in, JSON out, a timeout) minus the OAuth machinery.
package processor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultTimeout = 10 * time.Second

// IssueResult is what Cardnet hands back from POST /v1/cards/issue.
type IssueResult struct {
	CardToken string
	PANLast4  string
	Expiry    string
}

// AuthorizeResult is what Cardnet hands back from POST /v1/authorizations.
type AuthorizeResult struct {
	AuthCode string
	Status   string // "approved" or "declined"
}

// Processor is what cards-api's handlers need from Cardnet. Tests use a
// fake; CardnetClient is the real thing.
type Processor interface {
	IssueCard(ctx context.Context, accountID string) (IssueResult, error)
	Authorize(ctx context.Context, cardToken, amount, currency, merchantName string) (AuthorizeResult, error)
}

// CardnetClient is the real Processor, talking to CARDNET_URL with the
// X-Cardnet-Key header the vendor requires.
type CardnetClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewCardnetClient builds a CardnetClient against baseURL (staging or prod
// per deploy/values-<env>.yaml) authenticating with apiKey.
func NewCardnetClient(baseURL, apiKey string) *CardnetClient {
	return &CardnetClient{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// IssueCard calls Cardnet's POST /v1/cards/issue for account accountID.
func (c *CardnetClient) IssueCard(ctx context.Context, accountID string) (IssueResult, error) {
	req := struct {
		AccountID string `json:"account_id"`
	}{AccountID: accountID}

	var resp struct {
		CardToken string `json:"card_token"`
		PANLast4  string `json:"pan_last4"`
		Expiry    string `json:"expiry"`
	}
	if err := c.post(ctx, "/v1/cards/issue", req, &resp); err != nil {
		return IssueResult{}, fmt.Errorf("processor: issue card: %w", err)
	}

	return IssueResult{CardToken: resp.CardToken, PANLast4: resp.PANLast4, Expiry: resp.Expiry}, nil
}

// Authorize calls Cardnet's POST /v1/authorizations for a purchase against
// cardToken.
func (c *CardnetClient) Authorize(ctx context.Context, cardToken, amount, currency, merchantName string) (AuthorizeResult, error) {
	req := struct {
		CardToken    string `json:"card_token"`
		Amount       string `json:"amount"`
		Currency     string `json:"currency"`
		MerchantName string `json:"merchant_name"`
	}{CardToken: cardToken, Amount: amount, Currency: currency, MerchantName: merchantName}

	var resp struct {
		AuthCode string `json:"auth_code"`
		Status   string `json:"status"`
	}
	if err := c.post(ctx, "/v1/authorizations", req, &resp); err != nil {
		return AuthorizeResult{}, fmt.Errorf("processor: authorize: %w", err)
	}

	return AuthorizeResult{AuthCode: resp.AuthCode, Status: resp.Status}, nil
}

func (c *CardnetClient) post(ctx context.Context, path string, in, out interface{}) error {
	body, err := json.Marshal(in)
	if err != nil {
		return fmt.Errorf("encode request body: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Cardnet-Key", c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request cardnet: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("cardnet responded with status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response body: %w", err)
	}
	return nil
}
