CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS owners (
  id uuid PRIMARY KEY,
  email text UNIQUE NOT NULL,
  name text NOT NULL,
  phone text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS magic_links (
  id uuid PRIMARY KEY,
  owner_id uuid NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
  token text UNIQUE NOT NULL,
  expires_at timestamptz NOT NULL,
  used_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS chips (
  id uuid PRIMARY KEY,
  chip_id_raw text NOT NULL,
  chip_id_normalized text NOT NULL UNIQUE,
  manufacturer_hint text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_chips_normalized ON chips (chip_id_normalized);
CREATE INDEX IF NOT EXISTS idx_chips_raw ON chips (chip_id_raw);

CREATE TABLE IF NOT EXISTS pets (
  id uuid PRIMARY KEY,
  owner_id uuid NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
  chip_id uuid NOT NULL REFERENCES chips(id) ON DELETE CASCADE,
  pet_name text NOT NULL,
  species text NOT NULL CHECK (species IN ('dog', 'cat', 'other')),
  breed text,
  color text,
  date_of_birth date,
  notes text,
  photo_url text,
  active boolean NOT NULL DEFAULT true,
  registered_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (chip_id, owner_id)
);

CREATE TABLE IF NOT EXISTS lookups (
  id uuid PRIMARY KEY,
  chip_id_queried text NOT NULL,
  chip_id_normalized text NOT NULL,
  found boolean NOT NULL,
  looked_up_by_ip text,
  looked_up_by_agent text,
  notified_owner_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS transfers (
  id uuid PRIMARY KEY,
  chip_id uuid NOT NULL REFERENCES chips(id) ON DELETE CASCADE,
  from_owner_id uuid REFERENCES owners(id) ON DELETE SET NULL,
  to_owner_id uuid NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
  initiated_by text NOT NULL CHECK (initiated_by IN ('owner', 'shelter', 'vet')),
  initiator_note text,
  status text NOT NULL CHECK (status IN ('pending', 'approved', 'rejected', 'expired')),
  token text UNIQUE NOT NULL,
  expires_at timestamptz NOT NULL,
  resolved_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS disputes (
  id uuid PRIMARY KEY,
  chip_id uuid NOT NULL REFERENCES chips(id) ON DELETE CASCADE,
  reporter_email text NOT NULL,
  reporter_name text NOT NULL,
  description text NOT NULL,
  status text NOT NULL CHECK (status IN ('open', 'reviewing', 'resolved')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
