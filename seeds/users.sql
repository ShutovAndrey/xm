-- Test users for local development. Not for production.
-- password = crypt(plain_password, salt), bcrypt via pgcrypto.
INSERT INTO users (login, salt, password)
SELECT u.login, s.salt, crypt(u.password, s.salt)
FROM (VALUES
    ('alice', 'alice123'),
    ('bob',   'bob123'),
    ('carol', 'carol123')
) AS u(login, password)
CROSS JOIN LATERAL (SELECT gen_salt('bf') AS salt) AS s
ON CONFLICT (login) DO NOTHING;
