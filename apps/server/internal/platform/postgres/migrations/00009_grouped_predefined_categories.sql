-- +goose Up
-- Predefined categories were a flat list, so a workspace met two dozen equally weighted tiles
-- with no shape to them. They are now organized into groups: an ordinary category each member
-- hangs under, which a client renders as a section heading and which budgets and reports roll
-- up through. Existing workspaces are rearranged here rather than left behind, because a
-- hierarchy that only new workspaces receive would divide the product against itself.
--
-- Category identifiers never move, so every transaction allocation, budget item, and report
-- continues to resolve exactly as before.

ALTER TABLE categories DROP CONSTRAINT categories_predefined_key_valid;

ALTER TABLE categories
    ADD CONSTRAINT categories_predefined_key_valid CHECK (
        predefined_key IS NULL OR predefined_key IN (
            'group_food', 'group_home', 'group_transport', 'group_entertainment',
            'group_lifestyle', 'group_misc', 'group_income',
            'housing', 'groceries', 'dining', 'transportation', 'utilities', 'shopping',
            'health', 'entertainment', 'subscriptions', 'travel', 'education',
            'personal_care', 'gifts', 'other', 'salary', 'freelance', 'investment',
            'rental_income', 'gift_income', 'refund', 'other_income'
        )
    );

-- Create each group in every workspace that already holds predefined categories. A workspace
-- that somehow has one already keeps it, so re-running is harmless.
INSERT INTO categories (workspace_id, name, kind, predefined_key, icon, icon_type, icon_value, color_key)
SELECT DISTINCT existing.workspace_id, groups.key, groups.kind, groups.key,
       groups.icon, 'system', groups.icon, groups.color
FROM categories existing
CROSS JOIN (
    VALUES
        ('group_food', 'expense', 'utensils', 'blue'),
        ('group_home', 'expense', 'home', 'orange'),
        ('group_transport', 'expense', 'car', 'purple'),
        ('group_entertainment', 'expense', 'gamepad', 'pink'),
        ('group_lifestyle', 'expense', 'heart', 'red'),
        ('group_misc', 'expense', 'ellipsis', 'slate'),
        ('group_income', 'income', 'wallet', 'green')
) AS groups(key, kind, icon, color)
WHERE existing.predefined_key IS NOT NULL
ON CONFLICT (workspace_id, predefined_key) WHERE predefined_key IS NOT NULL DO NOTHING;

-- Move each predefined member under its group, and adopt the group's colour so a section reads
-- as one band. Only a member still sitting at the top level is moved: a workspace that already
-- arranged its own hierarchy has made a decision this migration must not overrule.
UPDATE categories AS member
SET parent_id = parent.id,
    color_key = parent.color_key,
    updated_at = now()
FROM categories AS parent
CROSS JOIN (
    VALUES
        ('groceries', 'group_food'), ('dining', 'group_food'),
        ('housing', 'group_home'), ('utilities', 'group_home'),
        ('transportation', 'group_transport'),
        ('entertainment', 'group_entertainment'), ('subscriptions', 'group_entertainment'),
        ('travel', 'group_entertainment'),
        ('health', 'group_lifestyle'), ('personal_care', 'group_lifestyle'),
        ('shopping', 'group_lifestyle'), ('gifts', 'group_lifestyle'),
        ('education', 'group_lifestyle'),
        ('other', 'group_misc'),
        ('salary', 'group_income'), ('freelance', 'group_income'),
        ('investment', 'group_income'), ('rental_income', 'group_income'),
        ('gift_income', 'group_income'), ('refund', 'group_income'),
        ('other_income', 'group_income')
) AS membership(member_key, group_key)
WHERE member.predefined_key = membership.member_key
  AND member.parent_id IS NULL
  AND parent.workspace_id = member.workspace_id
  AND parent.predefined_key = membership.group_key;

-- +goose Down
-- Return the members to the top level before the groups can go, so no category is orphaned.
UPDATE categories AS member
SET parent_id = NULL, updated_at = now()
FROM categories AS parent
WHERE member.parent_id = parent.id
  AND parent.predefined_key IN (
      'group_food', 'group_home', 'group_transport', 'group_entertainment',
      'group_lifestyle', 'group_misc', 'group_income'
  );

DELETE FROM categories
WHERE predefined_key IN (
    'group_food', 'group_home', 'group_transport', 'group_entertainment',
    'group_lifestyle', 'group_misc', 'group_income'
);

ALTER TABLE categories DROP CONSTRAINT categories_predefined_key_valid;

ALTER TABLE categories
    ADD CONSTRAINT categories_predefined_key_valid CHECK (
        predefined_key IS NULL OR predefined_key IN (
            'housing', 'groceries', 'dining', 'transportation', 'utilities', 'shopping',
            'health', 'entertainment', 'subscriptions', 'travel', 'education',
            'personal_care', 'gifts', 'other', 'salary', 'freelance', 'investment',
            'rental_income', 'gift_income', 'refund', 'other_income'
        )
    );
