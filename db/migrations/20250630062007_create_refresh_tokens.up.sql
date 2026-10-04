-- Таблица refresh-токенов с поддержкой ротации.
--
-- Хранится только хеш токена: утечка базы не даёт войти под пользователем,
-- потому что сам токен ещё нужно предъявить.
--
-- family_id связывает все токены одной сессии. При попытке воспользоваться
-- уже отозванным токеном отзывается всё семейство — это защита от повторного
-- использования токена после его утечки.
CREATE TABLE refresh_tokens (
    uuid       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_uuid  UUID NOT NULL REFERENCES users (uuid) ON DELETE CASCADE,
    family_id  TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    rotated_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

-- Хеш уникален: по нему ищется токен при проверке и ротации.
CREATE UNIQUE INDEX refresh_tokens_hash_key ON refresh_tokens (token_hash);

-- Ротация не перезаписывает хеш, а добавляет новую запись и помечает
-- прежнюю отозванной. Благодаря этому хеш предъявленного токена остаётся
-- в базе, и его повторное предъявление можно распознать. Этот индекс
-- обслуживает именно такую проверку.
CREATE INDEX refresh_tokens_active_idx ON refresh_tokens (token_hash) WHERE revoked_at IS NULL;

-- Отзыв всех токенов пользователя идёт по этому индексу.
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_uuid) WHERE revoked_at IS NULL;

-- Отзыв семейства идёт по этому индексу.
CREATE INDEX refresh_tokens_family_idx ON refresh_tokens (family_id) WHERE revoked_at IS NULL;

-- Очистка истёкших токенов идёт по этому индексу.
CREATE INDEX refresh_tokens_expires_idx ON refresh_tokens (expires_at);