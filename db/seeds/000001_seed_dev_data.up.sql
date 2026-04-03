INSERT INTO owners (id, email, name, phone, created_at, updated_at)
VALUES
  ('11111111-1111-1111-1111-111111111111', 'alex@example.com', 'Alex Rivera', '555-111-2222', now(), now()),
  ('22222222-2222-2222-2222-222222222222', 'sam@example.com', 'Sam Patel', '555-333-4444', now(), now())
ON CONFLICT (email) DO NOTHING;

INSERT INTO chips (id, chip_id_raw, chip_id_normalized, manufacturer_hint, created_at)
VALUES
  ('aaaaaaa1-aaaa-aaaa-aaaa-aaaaaaaaaaa1', '985000000000101', '985000000000101', 'HomeAgain', now()),
  ('aaaaaaa2-aaaa-aaaa-aaaa-aaaaaaaaaaa2', '982000000000202', '982000000000202', '24PetWatch / Allflex', now()),
  ('aaaaaaa3-aaaa-aaaa-aaaa-aaaaaaaaaaa3', '981000000000303', '981000000000303', 'Datamars / PetLink / Bayer ResQ', now()),
  ('aaaaaaa4-aaaa-aaaa-aaaa-aaaaaaaaaaa4', '900000000000404', '900000000000404', 'Trovan/AKC', now()),
  ('aaaaaaa5-aaaa-aaaa-aaaa-aaaaaaaaaaa5', '123456789', '000000123456789', 'Unknown manufacturer', now())
ON CONFLICT (chip_id_normalized) DO NOTHING;

INSERT INTO pets (id, owner_id, chip_id, pet_name, species, breed, color, active, registered_at, updated_at)
VALUES
  ('bbbbbbb1-bbbb-bbbb-bbbb-bbbbbbbbbbb1', '11111111-1111-1111-1111-111111111111', 'aaaaaaa1-aaaa-aaaa-aaaa-aaaaaaaaaaa1', 'Milo', 'dog', 'Retriever Mix', 'Golden', true, now(), now()),
  ('bbbbbbb2-bbbb-bbbb-bbbb-bbbbbbbbbbb2', '11111111-1111-1111-1111-111111111111', 'aaaaaaa3-aaaa-aaaa-aaaa-aaaaaaaaaaa3', 'Juniper', 'cat', 'Tabby', 'Brown', true, now(), now()),
  ('bbbbbbb3-bbbb-bbbb-bbbb-bbbbbbbbbbb3', '22222222-2222-2222-2222-222222222222', 'aaaaaaa5-aaaa-aaaa-aaaa-aaaaaaaaaaa5', 'Pip', 'other', 'Rabbit', 'White', true, now(), now())
ON CONFLICT (chip_id, owner_id) DO NOTHING;

INSERT INTO lookups (id, chip_id_queried, chip_id_normalized, found, looked_up_by_ip, looked_up_by_agent, created_at)
VALUES
  ('ccccccc1-cccc-cccc-cccc-ccccccccccc1', '985000000000101', '985000000000101', true, '127.0.0.1', 'SeedShelter', now() - interval '2 days'),
  ('ccccccc2-cccc-cccc-cccc-ccccccccccc2', '123456789', '000000123456789', true, '127.0.0.1', 'Public lookup', now() - interval '1 day'),
  ('ccccccc3-cccc-cccc-cccc-ccccccccccc3', '982000000000202', '982000000000202', false, '127.0.0.1', 'SeedShelter', now() - interval '3 hours')
ON CONFLICT (id) DO NOTHING;
