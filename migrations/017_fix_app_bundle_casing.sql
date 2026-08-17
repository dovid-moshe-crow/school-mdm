-- iOS allowListedAppBundleIDs is case-sensitive. We previously lowercased
-- bundle IDs; restore known App Store casings and keep metadata in sync.

UPDATE allowlist_entries
SET value = 'ph.telegra.Telegraph'
WHERE kind = 'app' AND lower(value) = 'ph.telegra.telegraph' AND value <> 'ph.telegra.Telegraph';

UPDATE allowlist_entries
SET value = 'net.whatsapp.WhatsApp'
WHERE kind = 'app' AND lower(value) = 'net.whatsapp.whatsapp' AND value <> 'net.whatsapp.WhatsApp';

UPDATE grants
SET value = 'ph.telegra.Telegraph'
WHERE kind = 'app' AND lower(value) = 'ph.telegra.telegraph' AND value <> 'ph.telegra.Telegraph';

UPDATE grants
SET value = 'net.whatsapp.WhatsApp'
WHERE kind = 'app' AND lower(value) = 'net.whatsapp.whatsapp' AND value <> 'net.whatsapp.WhatsApp';

UPDATE whitelist_pack_items
SET value = 'ph.telegra.Telegraph'
WHERE kind = 'app' AND lower(value) = 'ph.telegra.telegraph' AND value <> 'ph.telegra.Telegraph';

UPDATE whitelist_pack_items
SET value = 'net.whatsapp.WhatsApp'
WHERE kind = 'app' AND lower(value) = 'net.whatsapp.whatsapp' AND value <> 'net.whatsapp.WhatsApp';

UPDATE requests
SET value = 'ph.telegra.Telegraph'
WHERE target_kind = 'app' AND lower(value) = 'ph.telegra.telegraph' AND value <> 'ph.telegra.Telegraph';

UPDATE requests
SET value = 'net.whatsapp.WhatsApp'
WHERE target_kind = 'app' AND lower(value) = 'net.whatsapp.whatsapp' AND value <> 'net.whatsapp.WhatsApp';

-- app_metadata primary key is bundle_id; rewrite via delete+insert when needed.
INSERT INTO app_metadata (bundle_id, track_id, name, artist, artwork_url, store_url, updated_at, details)
SELECT 'ph.telegra.Telegraph', track_id, name, artist, artwork_url, store_url, updated_at, details
FROM app_metadata
WHERE lower(bundle_id) = 'ph.telegra.telegraph' AND bundle_id <> 'ph.telegra.Telegraph'
ON CONFLICT (bundle_id) DO UPDATE SET
  track_id = EXCLUDED.track_id,
  name = EXCLUDED.name,
  artist = EXCLUDED.artist,
  artwork_url = EXCLUDED.artwork_url,
  store_url = EXCLUDED.store_url,
  updated_at = EXCLUDED.updated_at,
  details = EXCLUDED.details;
DELETE FROM app_metadata WHERE bundle_id = 'ph.telegra.telegraph';

INSERT INTO app_metadata (bundle_id, track_id, name, artist, artwork_url, store_url, updated_at, details)
SELECT 'net.whatsapp.WhatsApp', track_id, name, artist, artwork_url, store_url, updated_at, details
FROM app_metadata
WHERE lower(bundle_id) = 'net.whatsapp.whatsapp' AND bundle_id <> 'net.whatsapp.WhatsApp'
ON CONFLICT (bundle_id) DO UPDATE SET
  track_id = EXCLUDED.track_id,
  name = EXCLUDED.name,
  artist = EXCLUDED.artist,
  artwork_url = EXCLUDED.artwork_url,
  store_url = EXCLUDED.store_url,
  updated_at = EXCLUDED.updated_at,
  details = EXCLUDED.details;
DELETE FROM app_metadata WHERE bundle_id = 'net.whatsapp.whatsapp';
