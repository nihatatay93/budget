-- +goose Up
ALTER TABLE categories
    ADD COLUMN icon_type TEXT NOT NULL DEFAULT 'system',
    ADD COLUMN icon_value TEXT NOT NULL DEFAULT 'ellipsis',
    ADD COLUMN color_key TEXT NOT NULL DEFAULT 'slate';

ALTER TABLE categories
    ADD CONSTRAINT categories_icon_type_valid CHECK (icon_type IN ('system', 'emoji')),
    ADD CONSTRAINT categories_system_icon_value_valid CHECK (
        icon_type <> 'system' OR icon_value IN (
            'home', 'shopping-cart', 'utensils', 'car', 'receipt', 'shopping-bag',
            'heart', 'gamepad', 'repeat', 'plane', 'graduation-cap', 'sparkles',
            'gift', 'ellipsis', 'wallet', 'laptop', 'trending-up', 'building', 'refund',
            'wallet-more'
        )
    ),
    ADD CONSTRAINT categories_color_key_valid CHECK (
        color_key IN ('green', 'mint', 'blue', 'cyan', 'purple', 'pink', 'red', 'orange', 'amber', 'slate')
    );

-- Preserve known legacy semantic icons and Unicode emoji. Other historical free-form values
-- become the safe fallback rather than preventing a migration of financial records.
UPDATE categories
SET icon_type = CASE
        WHEN icon IN (
            'home', 'shopping-cart', 'utensils', 'car', 'receipt', 'shopping-bag',
            'heart', 'gamepad', 'repeat', 'plane', 'graduation-cap', 'gift', 'wallet',
            'laptop', 'building'
        ) THEN 'system'
        WHEN icon IS NOT NULL AND icon ~ '[^[:ascii:]]' THEN 'emoji'
        ELSE 'system'
    END,
    icon_value = CASE
        WHEN icon IN (
            'home', 'shopping-cart', 'utensils', 'car', 'receipt', 'shopping-bag',
            'heart', 'gamepad', 'repeat', 'plane', 'graduation-cap', 'gift', 'wallet',
            'laptop', 'building'
        ) THEN icon
        WHEN icon IS NOT NULL AND icon ~ '[^[:ascii:]]' THEN icon
        ELSE 'ellipsis'
    END,
    color_key = 'slate';

-- Apply canonical predefined metadata to workspaces that already have their categories.
UPDATE categories
SET icon_type = 'system',
    icon_value = CASE predefined_key
        WHEN 'housing' THEN 'home'
        WHEN 'groceries' THEN 'shopping-cart'
        WHEN 'dining' THEN 'utensils'
        WHEN 'transportation' THEN 'car'
        WHEN 'utilities' THEN 'receipt'
        WHEN 'shopping' THEN 'shopping-bag'
        WHEN 'health' THEN 'heart'
        WHEN 'entertainment' THEN 'gamepad'
        WHEN 'subscriptions' THEN 'repeat'
        WHEN 'travel' THEN 'plane'
        WHEN 'education' THEN 'graduation-cap'
        WHEN 'personal_care' THEN 'sparkles'
        WHEN 'gifts' THEN 'gift'
        WHEN 'gift_income' THEN 'gift'
        WHEN 'other' THEN 'ellipsis'
        WHEN 'salary' THEN 'wallet'
        WHEN 'freelance' THEN 'laptop'
        WHEN 'investment' THEN 'trending-up'
        WHEN 'rental_income' THEN 'building'
        WHEN 'refund' THEN 'refund'
        WHEN 'other_income' THEN 'wallet-more'
    END,
    color_key = CASE predefined_key
        WHEN 'housing' THEN 'blue'
        WHEN 'travel' THEN 'blue'
        WHEN 'freelance' THEN 'blue'
        WHEN 'groceries' THEN 'green'
        WHEN 'salary' THEN 'green'
        WHEN 'investment' THEN 'green'
        WHEN 'dining' THEN 'orange'
        WHEN 'transportation' THEN 'cyan'
        WHEN 'rental_income' THEN 'cyan'
        WHEN 'refund' THEN 'cyan'
        WHEN 'utilities' THEN 'amber'
        WHEN 'shopping' THEN 'pink'
        WHEN 'personal_care' THEN 'pink'
        WHEN 'health' THEN 'red'
        WHEN 'gifts' THEN 'red'
        WHEN 'gift_income' THEN 'red'
        WHEN 'entertainment' THEN 'purple'
        WHEN 'subscriptions' THEN 'purple'
        WHEN 'education' THEN 'purple'
        WHEN 'other' THEN 'slate'
        WHEN 'other_income' THEN 'slate'
    END,
    icon = CASE predefined_key
        WHEN 'housing' THEN 'home'
        WHEN 'groceries' THEN 'shopping-cart'
        WHEN 'dining' THEN 'utensils'
        WHEN 'transportation' THEN 'car'
        WHEN 'utilities' THEN 'receipt'
        WHEN 'shopping' THEN 'shopping-bag'
        WHEN 'health' THEN 'heart'
        WHEN 'entertainment' THEN 'gamepad'
        WHEN 'subscriptions' THEN 'repeat'
        WHEN 'travel' THEN 'plane'
        WHEN 'education' THEN 'graduation-cap'
        WHEN 'personal_care' THEN 'sparkles'
        WHEN 'gifts' THEN 'gift'
        WHEN 'gift_income' THEN 'gift'
        WHEN 'other' THEN 'ellipsis'
        WHEN 'salary' THEN 'wallet'
        WHEN 'freelance' THEN 'laptop'
        WHEN 'investment' THEN 'trending-up'
        WHEN 'rental_income' THEN 'building'
        WHEN 'refund' THEN 'refund'
        WHEN 'other_income' THEN 'wallet-more'
    END
WHERE predefined_key IS NOT NULL;

-- +goose Down
ALTER TABLE categories
    DROP CONSTRAINT categories_color_key_valid,
    DROP CONSTRAINT categories_system_icon_value_valid,
    DROP CONSTRAINT categories_icon_type_valid,
    DROP COLUMN color_key,
    DROP COLUMN icon_value,
    DROP COLUMN icon_type;
