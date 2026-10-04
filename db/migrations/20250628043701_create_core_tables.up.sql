-- Базовые таблицы: пользователи, отзывы, заголовки, настройки, шаблоны.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Логины хранятся в приведённом к нижнему регистру виде, чтобы
-- "User" и "user" не считались разными учётными записями.
CREATE TABLE users (
    uuid         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    login        TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role         TEXT NOT NULL DEFAULT 'viewer',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT users_login_normalized CHECK (login = lower(login))
);

-- Уникальность логина без учёта регистра: логи хранятся уже в нижнем
-- регистре, поэтому обычный UNIQUE достаточно.
CREATE UNIQUE INDEX users_login_key ON users (lower(login));

CREATE TABLE review_titles (
    uuid        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    uuid_user   UUID NOT NULL REFERENCES users (uuid) ON DELETE CASCADE,
    uuid_review UUID NOT NULL,
    title       TEXT,
    request     TEXT,
    CONSTRAINT review_titles_user_review_key UNIQUE (uuid_user, uuid_review)
);

CREATE TABLE user_settings (
    uuid             UUID PRIMARY KEY REFERENCES users (uuid) ON DELETE CASCADE,
    main_prompt      TEXT NOT NULL DEFAULT '',
    memory_prompt    TEXT NOT NULL DEFAULT '',
    request          TEXT NOT NULL DEFAULT '',
    model            TEXT NOT NULL DEFAULT 'anthropic/claude-sonnet-4.6',
    memory           BOOLEAN NOT NULL DEFAULT FALSE,
    inprogress       TEXT NOT NULL DEFAULT '',
    processed_count  INT NOT NULL DEFAULT 1
);

CREATE TABLE reviews (
    uuid_uniq UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    uuid      UUID NOT NULL REFERENCES users (uuid) ON DELETE CASCADE,
    request   TEXT NOT NULL,
    answer    TEXT NOT NULL DEFAULT '',
    reasoning TEXT NOT NULL DEFAULT '',
    date      TIMESTAMPTZ NOT NULL DEFAULT now(),
    model     TEXT NOT NULL DEFAULT '',
    favorite  BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX reviews_user_date_idx ON reviews (uuid, date DESC);

CREATE TABLE custom_prompts (
    uuid_uniq UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    uuid_user UUID NOT NULL REFERENCES users (uuid) ON DELETE CASCADE,
    name      TEXT NOT NULL,
    prompt    TEXT NOT NULL DEFAULT ''
);

CREATE INDEX custom_prompts_user_idx ON custom_prompts (uuid_user);
