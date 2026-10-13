CREATE TYPE company_type AS ENUM (
    'Corporations',
    'NonProfit',
    'Cooperative',
    'Sole Proprietorship'
);

CREATE TABLE companies (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(15)  NOT NULL UNIQUE,
    description     TEXT,
    employees_count INTEGER      NOT NULL,
    is_registered   BOOLEAN      NOT NULL,
    type            company_type NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now()
);
