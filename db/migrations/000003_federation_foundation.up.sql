CREATE TABLE IF NOT EXISTS organizations (
  id uuid PRIMARY KEY,
  slug text NOT NULL UNIQUE,
  name text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS nodes (
  id uuid PRIMARY KEY,
  organization_id uuid NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
  slug text NOT NULL UNIQUE,
  display_name text NOT NULL,
  public_base_url text NOT NULL,
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'read_only', 'retired')),
  federation_mode text NOT NULL DEFAULT 'single_node_reference' CHECK (federation_mode IN ('single_node_reference', 'federated')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS owner_contacts (
  owner_id uuid PRIMARY KEY REFERENCES owners(id) ON DELETE CASCADE,
  email text NOT NULL UNIQUE,
  phone text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS pet_profiles (
  id uuid PRIMARY KEY,
  chip_id uuid NOT NULL REFERENCES chips(id) ON DELETE CASCADE,
  display_name text NOT NULL,
  species text NOT NULL CHECK (species IN ('dog', 'cat', 'other')),
  breed text,
  color text,
  date_of_birth date,
  notes text,
  photo_url text,
  public_contact_policy text NOT NULL DEFAULT 'mediated' CHECK (public_contact_policy IN ('mediated')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS registration_claims (
  id uuid PRIMARY KEY,
  chip_id uuid NOT NULL REFERENCES chips(id) ON DELETE CASCADE,
  pet_profile_id uuid NOT NULL REFERENCES pet_profiles(id) ON DELETE CASCADE,
  claimant_owner_id uuid NOT NULL REFERENCES owners(id) ON DELETE CASCADE,
  source_node_id uuid REFERENCES nodes(id) ON DELETE SET NULL,
  status text NOT NULL CHECK (status IN ('active', 'transferred', 'disputed', 'inactive')),
  claim_scope text NOT NULL DEFAULT 'ownership' CHECK (claim_scope IN ('ownership', 'caretaker', 'legacy_import')),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ownership_events (
  id uuid PRIMARY KEY,
  aggregate_type text NOT NULL,
  aggregate_id uuid NOT NULL,
  event_type text NOT NULL,
  payload_json jsonb NOT NULL,
  actor_type text NOT NULL,
  actor_id text NOT NULL,
  event_hash text NOT NULL UNIQUE,
  previous_event_hash text,
  signature text,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS actor_keys (
  id uuid PRIMARY KEY,
  actor_type text NOT NULL,
  actor_id text NOT NULL,
  node_id uuid REFERENCES nodes(id) ON DELETE SET NULL,
  algorithm text NOT NULL,
  public_key text NOT NULL,
  status text NOT NULL DEFAULT 'announced' CHECK (status IN ('announced', 'active', 'retired')),
  created_at timestamptz NOT NULL DEFAULT now(),
  retired_at timestamptz
);

CREATE TABLE IF NOT EXISTS public_snapshots (
  id uuid PRIMARY KEY,
  node_id uuid NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
  snapshot_type text NOT NULL CHECK (snapshot_type IN ('public_registry')),
  payload_json jsonb NOT NULL,
  payload_hash text NOT NULL,
  generated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS export_batches (
  id uuid PRIMARY KEY,
  node_id uuid NOT NULL REFERENCES nodes(id) ON DELETE CASCADE,
  export_type text NOT NULL CHECK (export_type IN ('owner_export', 'public_snapshot', 'event_stream')),
  scope text NOT NULL,
  payload_json jsonb NOT NULL,
  payload_hash text NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_nodes_org_id ON nodes (organization_id);
CREATE INDEX IF NOT EXISTS idx_owner_contacts_email ON owner_contacts (email);
CREATE INDEX IF NOT EXISTS idx_pet_profiles_chip_id ON pet_profiles (chip_id);
CREATE INDEX IF NOT EXISTS idx_registration_claims_chip_id ON registration_claims (chip_id);
CREATE INDEX IF NOT EXISTS idx_registration_claims_owner_id ON registration_claims (claimant_owner_id);
CREATE INDEX IF NOT EXISTS idx_registration_claims_status ON registration_claims (status);
CREATE INDEX IF NOT EXISTS idx_ownership_events_aggregate_created ON ownership_events (aggregate_type, aggregate_id, created_at);
CREATE INDEX IF NOT EXISTS idx_ownership_events_created_at ON ownership_events (created_at);
CREATE INDEX IF NOT EXISTS idx_public_snapshots_node_generated_at ON public_snapshots (node_id, generated_at DESC);
CREATE INDEX IF NOT EXISTS idx_export_batches_node_created_at ON export_batches (node_id, created_at DESC);

INSERT INTO organizations (id, slug, name, created_at)
VALUES ('11111111-1111-1111-1111-111111111111', 'openchip-reference', 'OpenChip Reference Operators', now())
ON CONFLICT (slug) DO NOTHING;

INSERT INTO nodes (id, organization_id, slug, display_name, public_base_url, status, federation_mode, created_at, updated_at)
VALUES (
  '22222222-2222-2222-2222-222222222222',
  '11111111-1111-1111-1111-111111111111',
  'local-reference-node',
  'OpenChip Local Reference Node',
  'http://localhost:8080',
  'active',
  'single_node_reference',
  now(),
  now()
)
ON CONFLICT (slug) DO NOTHING;

INSERT INTO actor_keys (id, actor_type, actor_id, node_id, algorithm, public_key, status, created_at)
VALUES (
  '33333333-3333-3333-3333-333333333333',
  'node',
  '22222222-2222-2222-2222-222222222222',
  '22222222-2222-2222-2222-222222222222',
  'unsigned-placeholder',
  'phase-1-placeholder',
  'announced',
  now()
)
ON CONFLICT DO NOTHING;

INSERT INTO owner_contacts (owner_id, email, phone, created_at, updated_at)
SELECT id, email, phone, created_at, updated_at
FROM owners
ON CONFLICT (owner_id) DO UPDATE SET
  email = EXCLUDED.email,
  phone = EXCLUDED.phone,
  updated_at = EXCLUDED.updated_at;

INSERT INTO pet_profiles (id, chip_id, display_name, species, breed, color, date_of_birth, notes, photo_url, public_contact_policy, created_at, updated_at)
SELECT
  id,
  chip_id,
  pet_name,
  species,
  breed,
  color,
  date_of_birth,
  notes,
  photo_url,
  'mediated',
  registered_at,
  updated_at
FROM pets
ON CONFLICT (id) DO NOTHING;

INSERT INTO registration_claims (id, chip_id, pet_profile_id, claimant_owner_id, source_node_id, status, claim_scope, created_at, updated_at)
SELECT
  gen_random_uuid(),
  p.chip_id,
  p.id,
  p.owner_id,
  '22222222-2222-2222-2222-222222222222',
  CASE WHEN p.active THEN 'active' ELSE 'inactive' END,
  'ownership',
  p.registered_at,
  p.updated_at
FROM pets p
ON CONFLICT DO NOTHING;
