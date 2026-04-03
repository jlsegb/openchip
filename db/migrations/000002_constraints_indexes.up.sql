ALTER TABLE chips
  ADD CONSTRAINT chips_chip_id_normalized_digits_chk
  CHECK (chip_id_normalized ~ '^\d{15}$');

ALTER TABLE magic_links
  ADD CONSTRAINT magic_links_expires_after_created_chk
  CHECK (expires_at > created_at);

ALTER TABLE transfers
  ADD CONSTRAINT transfers_expires_after_created_chk
  CHECK (expires_at > created_at);

CREATE INDEX IF NOT EXISTS idx_pets_owner_id ON pets (owner_id);
CREATE INDEX IF NOT EXISTS idx_pets_chip_id ON pets (chip_id);
CREATE INDEX IF NOT EXISTS idx_lookups_chip_created_at ON lookups (chip_id_normalized, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_lookups_created_at ON lookups (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_magic_links_token ON magic_links (token);
CREATE INDEX IF NOT EXISTS idx_transfers_token ON transfers (token);

COMMENT ON INDEX idx_pets_owner_id IS 'Supports active pet dashboard queries by owner';
COMMENT ON INDEX idx_pets_chip_id IS 'Supports pet-to-chip joins during lookups and transfers';
COMMENT ON INDEX idx_lookups_chip_created_at IS 'Supports per-chip history and recent stats queries';
COMMENT ON INDEX idx_lookups_created_at IS 'Supports time-range lookup reporting';
COMMENT ON INDEX idx_magic_links_token IS 'Supports magic link verification queries';
COMMENT ON INDEX idx_transfers_token IS 'Supports transfer confirmation queries';
