package httpapi

import (
	"context"
	"io"
	"log/slog"

	"github.com/demo-org-np-migration/cards-api/internal/processor"
	"github.com/demo-org-np-migration/cards-api/internal/store"
)

// fakeStore is the store.Store used by handler tests — no Postgres
// involved, per the spec's "tests with store/processor/ledger as
// interfaces with fakes" requirement.
type fakeStore struct {
	cards         map[string]store.Card
	byAccount     map[string][]store.Card
	authorizations []store.Authorization
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		cards:     map[string]store.Card{},
		byAccount: map[string][]store.Card{},
	}
}

func (f *fakeStore) CreateCard(ctx context.Context, card store.Card) (*store.Card, error) {
	f.cards[card.ID] = card
	f.byAccount[card.AccountID] = append(f.byAccount[card.AccountID], card)
	return &card, nil
}

func (f *fakeStore) GetCard(ctx context.Context, id string) (*store.Card, error) {
	c, ok := f.cards[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return &c, nil
}

func (f *fakeStore) ListCardsByAccount(ctx context.Context, accountID string) ([]store.Card, error) {
	return f.byAccount[accountID], nil
}

func (f *fakeStore) BlockCard(ctx context.Context, id string) (*store.Card, error) {
	c, ok := f.cards[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	c.Status = "blocked"
	f.cards[id] = c
	return &c, nil
}

func (f *fakeStore) CreateAuthorization(ctx context.Context, auth store.Authorization) (*store.Authorization, error) {
	f.authorizations = append(f.authorizations, auth)
	return &auth, nil
}

// fakeProcessor is the processor.Processor used by handler tests — no
// Cardnet involved.
type fakeProcessor struct {
	issueResult     processor.IssueResult
	issueErr        error
	authorizeResult processor.AuthorizeResult
	authorizeErr    error
	authorizeCalls  int
}

func (f *fakeProcessor) IssueCard(ctx context.Context, accountID string) (processor.IssueResult, error) {
	return f.issueResult, f.issueErr
}

func (f *fakeProcessor) Authorize(ctx context.Context, cardToken, amount, currency, merchantName string) (processor.AuthorizeResult, error) {
	f.authorizeCalls++
	return f.authorizeResult, f.authorizeErr
}

// fakeLedger is the ledger.Client used by handler tests — no ledger-core
// involved. It records every debit it's asked to record so tests can
// assert an approved authorization touches the ledger and a declined one
// doesn't.
type fakeLedger struct {
	debits []recordedDebit
	err    error
}

type recordedDebit struct {
	accountID string
	amount    string
	reference string
}

func (f *fakeLedger) RecordDebit(ctx context.Context, accountID, amount, reference string) error {
	if f.err != nil {
		return f.err
	}
	f.debits = append(f.debits, recordedDebit{accountID: accountID, amount: amount, reference: reference})
	return nil
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError + 1}))
}

func newHandlers(st store.Store, proc processor.Processor, ledgerClient *fakeLedger) *Handlers {
	return &Handlers{Store: st, Processor: proc, Ledger: ledgerClient, Logger: testLogger()}
}
