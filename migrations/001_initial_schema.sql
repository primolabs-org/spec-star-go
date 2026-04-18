CREATE TABLE clients (
    client_id       UUID        PRIMARY KEY,
    external_id     VARCHAR     NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE assets (
    asset_id           UUID        PRIMARY KEY,
    instrument_id      VARCHAR     NOT NULL,
    product_type       VARCHAR     NOT NULL CHECK (product_type IN ('CDB','LF','LCI','LCA','CRI','CRA','LFT')),
    offer_id           VARCHAR,
    emission_entity_id VARCHAR     NOT NULL,
    issuer_document_id VARCHAR,
    market_code        VARCHAR,
    asset_name         VARCHAR     NOT NULL,
    issuance_date      DATE        NOT NULL,
    maturity_date      DATE        NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_assets_instrument_id ON assets (instrument_id);

CREATE TABLE positions (
    position_id               UUID           PRIMARY KEY,
    client_id                 UUID           NOT NULL REFERENCES clients (client_id),
    asset_id                  UUID           NOT NULL REFERENCES assets (asset_id),
    amount                    NUMERIC(18,6)  NOT NULL CHECK (amount >= 0),
    unit_price                NUMERIC(20,8)  NOT NULL CHECK (unit_price >= 0),
    total_value               NUMERIC(20,8)  NOT NULL CHECK (total_value >= 0),
    collateral_value          NUMERIC(20,8)  NOT NULL DEFAULT 0 CHECK (collateral_value >= 0),
    judiciary_collateral_value NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (judiciary_collateral_value >= 0),
    created_at                TIMESTAMPTZ    NOT NULL DEFAULT now(),
    updated_at                TIMESTAMPTZ    NOT NULL DEFAULT now(),
    purchased_at              TIMESTAMPTZ    NOT NULL,
    row_version               INTEGER        NOT NULL DEFAULT 1 CHECK (row_version > 0),
    CHECK (total_value = amount * unit_price)
);

CREATE INDEX idx_positions_client_asset ON positions (client_id, asset_id);

CREATE TABLE processed_commands (
    command_id        UUID        PRIMARY KEY,
    command_type      VARCHAR     NOT NULL,
    order_id          VARCHAR     NOT NULL,
    client_id         UUID        NOT NULL,
    response_snapshot JSONB       NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_processed_commands_type_order ON processed_commands (command_type, order_id);
