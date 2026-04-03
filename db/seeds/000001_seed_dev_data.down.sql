DELETE FROM lookups
WHERE id IN (
  'ccccccc1-cccc-cccc-cccc-ccccccccccc1',
  'ccccccc2-cccc-cccc-cccc-ccccccccccc2',
  'ccccccc3-cccc-cccc-cccc-ccccccccccc3'
);

DELETE FROM pets
WHERE id IN (
  'bbbbbbb1-bbbb-bbbb-bbbb-bbbbbbbbbbb1',
  'bbbbbbb2-bbbb-bbbb-bbbb-bbbbbbbbbbb2',
  'bbbbbbb3-bbbb-bbbb-bbbb-bbbbbbbbbbb3'
);

DELETE FROM chips
WHERE id IN (
  'aaaaaaa1-aaaa-aaaa-aaaa-aaaaaaaaaaa1',
  'aaaaaaa2-aaaa-aaaa-aaaa-aaaaaaaaaaa2',
  'aaaaaaa3-aaaa-aaaa-aaaa-aaaaaaaaaaa3',
  'aaaaaaa4-aaaa-aaaa-aaaa-aaaaaaaaaaa4',
  'aaaaaaa5-aaaa-aaaa-aaaa-aaaaaaaaaaa5'
);

DELETE FROM owners
WHERE id IN (
  '11111111-1111-1111-1111-111111111111',
  '22222222-2222-2222-2222-222222222222'
);
