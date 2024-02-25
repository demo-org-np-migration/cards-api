package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Postgres is the real Store, backed by a connection pool to the `cards`
// database that cards-api owns.
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres builds a Postgres store from an already-open pool.
func NewPostgres(pool *pgxpool.Pool) *Postgres {
	return &Postgres{pool: pool}
}

// Connect opens a pgxpool against databaseURL and pings it once so startup
// fails fast if `cards` isn't reachable.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

const createCardQuery = `
INSERT INTO cards (id, account_id, card_token, pan_last4, expiry, status, created_at)
VALUES ($1, $2, $3, $4, $5, $6, now())
RETURNING id, account_id, card_token, pan_last4, expiry, status, created_at
`

// CreateCard persists a card just issued by Cardnet.
func (p *Postgres) CreateCard(ctx context.Context, card Card) (*Card, error) {
	row := p.pool.QueryRow(ctx, createCardQuery,
		card.ID, card.AccountID, card.CardToken, card.PANLast4, card.Expiry, card.Status)

	var c Card
	err := row.Scan(&c.ID, &c.AccountID, &c.CardToken, &c.PANLast4, &c.Expiry, &c.Status, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

const getCardQuery = `
SELECT id, account_id, card_token, pan_last4, expiry, status, created_at
FROM cards
WHERE id = $1
`

// GetCard backs both GET /v1/cards/{id} and the internal lookup that
// POST /v1/authorizations does to find the card being charged.
func (p *Postgres) GetCard(ctx context.Context, id string) (*Card, error) {
	row := p.pool.QueryRow(ctx, getCardQuery, id)

	var c Card
	err := row.Scan(&c.ID, &c.AccountID, &c.CardToken, &c.PANLast4, &c.Expiry, &c.Status, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

const listCardsByAccountQuery = `
SELECT id, account_id, card_token, pan_last4, expiry, status, created_at
FROM cards
WHERE account_id = $1
`

// ListCardsByAccount backs GET /v1/accounts/{account_id}/cards.
func (p *Postgres) ListCardsByAccount(ctx context.Context, accountID string) ([]Card, error) {
	rows, err := p.pool.Query(ctx, listCardsByAccountQuery, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cards := []Card{}
	for rows.Next() {
		var c Card
		if err := rows.Scan(&c.ID, &c.AccountID, &c.CardToken, &c.PANLast4, &c.Expiry, &c.Status, &c.CreatedAt); err != nil {
			return nil, err
		}
		cards = append(cards, c)
	}
	return cards, rows.Err()
}

const blockCardQuery = `
UPDATE cards
SET status = 'blocked'
WHERE id = $1
RETURNING id, account_id, card_token, pan_last4, expiry, status, created_at
`

// BlockCard backs POST /v1/cards/{id}/block.
func (p *Postgres) BlockCard(ctx context.Context, id string) (*Card, error) {
	row := p.pool.QueryRow(ctx, blockCardQuery, id)

	var c Card
	err := row.Scan(&c.ID, &c.AccountID, &c.CardToken, &c.PANLast4, &c.Expiry, &c.Status, &c.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

const createAuthorizationQuery = `
INSERT INTO authorizations (id, card_id, amount, currency, merchant_name, auth_code, status, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
RETURNING id, card_id, amount::text, currency, merchant_name, auth_code, status, created_at
`

// CreateAuthorization persists the result of a POST /v1/authorizations
// attempt, approved or declined.
func (p *Postgres) CreateAuthorization(ctx context.Context, auth Authorization) (*Authorization, error) {
	row := p.pool.QueryRow(ctx, createAuthorizationQuery,
		auth.ID, auth.CardID, auth.Amount, auth.Currency, auth.MerchantName, auth.AuthCode, auth.Status)

	var a Authorization
	var authCode *string
	err := row.Scan(&a.ID, &a.CardID, &a.Amount, &a.Currency, &a.MerchantName, &authCode, &a.Status, &a.CreatedAt)
	if err != nil {
		return nil, err
	}
	if authCode != nil {
		a.AuthCode = *authCode
	}
	return &a, nil
}
