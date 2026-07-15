-- Migrate legacy access_requests → requests (safe if legacy table missing)

DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = 'public' AND table_name = 'access_requests'
  ) AND EXISTS (
    SELECT 1 FROM information_schema.tables
    WHERE table_schema = 'public' AND table_name = 'requests'
  ) THEN
    INSERT INTO requests (id, type, target_kind, value, enrollment_id, reason, status, duration, created_at, decided_at)
    SELECT
      id,
      'access',
      kind,
      value,
      enrollment_id,
      reason,
      status,
      duration,
      created_at,
      decided_at
    FROM access_requests
    ON CONFLICT (id) DO NOTHING;
  END IF;
END $$;
