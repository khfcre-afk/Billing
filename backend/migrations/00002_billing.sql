-- +goose Up
-- +goose StatementBegin
CREATE TABLE tariffs (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug            TEXT NOT NULL UNIQUE,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    price_cents     BIGINT NOT NULL CHECK (price_cents >= 0),
    daily_cost_cents BIGINT NOT NULL CHECK (daily_cost_cents >= 0),
    cpu_limit       INTEGER NOT NULL DEFAULT 100,
    memory_mb       INTEGER NOT NULL,
    disk_mb         INTEGER NOT NULL,
    swap_mb         INTEGER NOT NULL DEFAULT 0,
    io_weight       INTEGER NOT NULL DEFAULT 500,
    egg_id          INTEGER NOT NULL,
    nest_id         INTEGER NOT NULL,
    docker_image    TEXT NOT NULL,
    startup_command TEXT NOT NULL,
    environment     JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TYPE promo_kind AS ENUM ('balance','tariff_grant','percent_off');

CREATE TABLE promo_codes (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code            TEXT NOT NULL UNIQUE,
    kind            promo_kind NOT NULL,
    balance_cents   BIGINT,
    tariff_id       UUID REFERENCES tariffs(id) ON DELETE SET NULL,
    grant_days      INTEGER,
    percent_off     INTEGER CHECK (percent_off IS NULL OR (percent_off > 0 AND percent_off <= 100)),
    max_uses        INTEGER,
    uses_count      INTEGER NOT NULL DEFAULT 0,
    per_user_limit  INTEGER NOT NULL DEFAULT 1,
    expires_at      TIMESTAMPTZ,
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT promo_kind_fields_ck CHECK (
        (kind = 'balance'      AND balance_cents IS NOT NULL AND balance_cents > 0)
     OR (kind = 'tariff_grant' AND tariff_id IS NOT NULL AND grant_days IS NOT NULL AND grant_days > 0)
     OR (kind = 'percent_off'  AND percent_off IS NOT NULL)
    )
);

CREATE TABLE promo_redemptions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    promo_id        UUID NOT NULL REFERENCES promo_codes(id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (promo_id, user_id)
);

CREATE TYPE transaction_kind AS ENUM (
    'promo_deposit',
    'promo_tariff_grant',
    'tariff_purchase',
    'daily_charge',
    'refund',
    'admin_adjustment'
);

CREATE TABLE transactions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kind            transaction_kind NOT NULL,
    amount_cents    BIGINT NOT NULL,
    balance_after   BIGINT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    promo_id        UUID REFERENCES promo_codes(id) ON DELETE SET NULL,
    server_id       UUID,
    tariff_id       UUID REFERENCES tariffs(id) ON DELETE SET NULL,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX transactions_user_created_idx ON transactions (user_id, created_at DESC);

CREATE TYPE server_status AS ENUM ('provisioning','active','suspended','deleted');

CREATE TABLE servers (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tariff_id           UUID NOT NULL REFERENCES tariffs(id),
    ptero_server_id     INTEGER UNIQUE,
    ptero_identifier    TEXT,
    name                TEXT NOT NULL,
    status              server_status NOT NULL DEFAULT 'provisioning',
    paid_until          TIMESTAMPTZ NOT NULL,
    last_charged_at     TIMESTAMPTZ,
    allocation_ip       TEXT,
    allocation_port     INTEGER,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX servers_user_idx ON servers (user_id);
CREATE INDEX servers_status_idx ON servers (status);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE servers;
DROP TYPE server_status;
DROP TABLE transactions;
DROP TYPE transaction_kind;
DROP TABLE promo_redemptions;
DROP TABLE promo_codes;
DROP TYPE promo_kind;
DROP TABLE tariffs;
-- +goose StatementEnd
