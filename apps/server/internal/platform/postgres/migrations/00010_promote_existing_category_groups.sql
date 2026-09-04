-- +goose Up
-- Migration 00009 introduced a group above every predefined category, including three that only
-- restated a category already present: Transport above Transportation, Entertainment above
-- Entertainment, and Miscellaneous above Other. In the picker those rendered as two tiles of the
-- same name inside one section, and in Turkish the pair was identical — Ulaşım above Ulaşım.
--
-- A group is an ordinary category, so where one already names the idea it becomes the group
-- rather than gaining a synonym above it. Subscriptions and Travel move under Entertainment;
-- Transportation and Other return to the top level and stand alone there.

UPDATE categories AS member
SET parent_id = entertainment.id,
    color_key = entertainment.color_key,
    updated_at = now()
FROM categories AS parent
JOIN categories AS entertainment
  ON entertainment.workspace_id = parent.workspace_id
 AND entertainment.predefined_key = 'entertainment'
WHERE member.parent_id = parent.id
  AND parent.predefined_key = 'group_entertainment'
  AND member.predefined_key IN ('subscriptions', 'travel');

-- Everything else that hung under one of the three redundant groups — including Entertainment
-- itself, and any category a workspace filed there by hand — returns to the top level.
UPDATE categories AS member
SET parent_id = NULL, updated_at = now()
FROM categories AS parent
WHERE member.parent_id = parent.id
  AND parent.predefined_key IN ('group_entertainment', 'group_transport', 'group_misc');

-- Remove the emptied groups. A group a workspace has already allocated to, budgeted against, or
-- given children of its own is left exactly where it is: this migration corrects a naming
-- mistake and must not delete a category that now carries financial meaning.
DELETE FROM categories AS redundant
WHERE redundant.predefined_key IN ('group_entertainment', 'group_transport', 'group_misc')
  AND NOT EXISTS (
      SELECT 1 FROM transaction_allocations allocation
      WHERE allocation.category_id = redundant.id
  )
  AND NOT EXISTS (
      SELECT 1 FROM budget_items item
      WHERE item.category_id = redundant.id
  )
  AND NOT EXISTS (
      SELECT 1 FROM categories child WHERE child.parent_id = redundant.id
  );

-- +goose Down
-- Recreate the three groups and return their members, mirroring 00009's arrangement.
INSERT INTO categories (workspace_id, name, kind, predefined_key, icon, icon_type, icon_value, color_key)
SELECT DISTINCT existing.workspace_id, groups.key, 'expense', groups.key,
       groups.icon, 'system', groups.icon, groups.color
FROM categories existing
CROSS JOIN (
    VALUES
        ('group_transport', 'car', 'purple'),
        ('group_entertainment', 'gamepad', 'pink'),
        ('group_misc', 'ellipsis', 'slate')
) AS groups(key, icon, color)
WHERE existing.predefined_key IS NOT NULL
ON CONFLICT (workspace_id, predefined_key) WHERE predefined_key IS NOT NULL DO NOTHING;

UPDATE categories AS member
SET parent_id = parent.id, color_key = parent.color_key, updated_at = now()
FROM categories AS parent
CROSS JOIN (
    VALUES
        ('transportation', 'group_transport'),
        ('entertainment', 'group_entertainment'),
        ('subscriptions', 'group_entertainment'),
        ('travel', 'group_entertainment'),
        ('other', 'group_misc')
) AS membership(member_key, group_key)
WHERE member.predefined_key = membership.member_key
  AND parent.workspace_id = member.workspace_id
  AND parent.predefined_key = membership.group_key;
