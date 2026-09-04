-- +goose Up
ALTER TABLE categories
    ADD COLUMN predefined_key TEXT;

ALTER TABLE categories
    ADD CONSTRAINT categories_predefined_key_valid CHECK (
        predefined_key IS NULL OR predefined_key IN (
            'housing', 'groceries', 'dining', 'transportation', 'utilities', 'shopping',
            'health', 'entertainment', 'subscriptions', 'travel', 'education',
            'personal_care', 'gifts', 'other', 'salary', 'freelance', 'investment',
            'rental_income', 'gift_income', 'refund', 'other_income'
        )
    );

CREATE UNIQUE INDEX categories_predefined_key_unique
    ON categories (workspace_id, predefined_key)
    WHERE predefined_key IS NOT NULL;

-- +goose Down
DROP INDEX categories_predefined_key_unique;
ALTER TABLE categories DROP CONSTRAINT categories_predefined_key_valid;
ALTER TABLE categories DROP COLUMN predefined_key;
