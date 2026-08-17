-- Drop unused admin undo stack (audit trail lives in activity_events).

DROP INDEX IF EXISTS admin_actions_undone_idx;
DROP INDEX IF EXISTS admin_actions_at_idx;
DROP TABLE IF EXISTS admin_actions;
