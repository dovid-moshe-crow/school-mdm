-- Built-in Apple/system apps applied to every device (hidden from pack lists).

CREATE TABLE IF NOT EXISTS system_allowlist (
    kind       TEXT NOT NULL CHECK (kind IN ('app', 'url')),
    value      TEXT NOT NULL,
    enabled    BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (kind, value)
);

-- Move Apple system bundle IDs out of the imported main allowlist pack.
INSERT INTO system_allowlist (kind, value)
SELECT DISTINCT i.kind, i.value
FROM whitelist_pack_items i
JOIN whitelist_packs p ON p.id = i.pack_id
WHERE p.name = 'רשימת אפליקציות מותרות'
  AND i.kind = 'app'
  AND lower(i.value) LIKE 'com.apple.%'
  AND lower(i.value) NOT IN (
        'com.apple.mobilesafari',
        'com.apple.webapp'
      )
ON CONFLICT DO NOTHING;

DELETE FROM whitelist_pack_items i
USING whitelist_packs p
WHERE i.pack_id = p.id
  AND p.name = 'רשימת אפליקציות מותרות'
  AND i.kind = 'app'
  AND lower(i.value) LIKE 'com.apple.%'
  AND lower(i.value) NOT IN (
        'com.apple.mobilesafari',
        'com.apple.webapp'
      );
