-- Память пользователя: факты о его предпочтениях, которые модель учитывает
-- при генерации отзывов.
--
-- Одна запись на пользователя: uuid одновременно служит первичным ключом и
-- внешним ключом, поэтому отдельного идентификатора не требуется.
CREATE TABLE user_memory (
    uuid       UUID PRIMARY KEY REFERENCES users (uuid) ON DELETE CASCADE,
    memory     TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
