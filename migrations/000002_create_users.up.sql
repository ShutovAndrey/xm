CREATE TABLE users (
    id       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    salt     VARCHAR(4) NOT NULL,
    password TEXT NOT NULL
);
