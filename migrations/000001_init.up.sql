-- Core schema owned by cards-api (las convenciones internas de API).
CREATE TABLE cards (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    card_token TEXT NOT NULL,
    pan_last4 CHAR(4) NOT NULL,
    expiry CHAR(5) NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('active', 'blocked')),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

CREATE TABLE authorizations (
    id UUID PRIMARY KEY,
    card_id UUID NOT NULL REFERENCES cards (id),
    amount NUMERIC(18, 2) NOT NULL,
    currency CHAR(3) NOT NULL,
    merchant_name TEXT NOT NULL,
    auth_code TEXT,
    status TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

CREATE INDEX idx_cards_account_id ON cards (account_id);
CREATE INDEX idx_authorizations_card_id ON authorizations (card_id);
