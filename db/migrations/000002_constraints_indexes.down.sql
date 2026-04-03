DROP INDEX IF EXISTS idx_transfers_token;
DROP INDEX IF EXISTS idx_magic_links_token;
DROP INDEX IF EXISTS idx_lookups_created_at;
DROP INDEX IF EXISTS idx_lookups_chip_created_at;
DROP INDEX IF EXISTS idx_pets_chip_id;
DROP INDEX IF EXISTS idx_pets_owner_id;

ALTER TABLE transfers
  DROP CONSTRAINT IF EXISTS transfers_expires_after_created_chk;

ALTER TABLE magic_links
  DROP CONSTRAINT IF EXISTS magic_links_expires_after_created_chk;

ALTER TABLE chips
  DROP CONSTRAINT IF EXISTS chips_chip_id_normalized_digits_chk;
