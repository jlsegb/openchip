package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/openchip/openchip/api/internal/chip"
	"github.com/openchip/openchip/api/internal/store"
	"github.com/openchip/openchip/api/internal/testutil"
)

func TestLookupRegistrationsHitsChipIDRawWhenNormalizedDiffers(t *testing.T) {
	env := testutil.StartPostgres(t)
	s := store.New(env.Pool, 5*time.Second)

	ownerID := uuid.NewString()
	chipID := uuid.NewString()
	petID := uuid.NewString()
	if _, err := env.Pool.Exec(context.Background(), `
		INSERT INTO owners (id, email, name, created_at, updated_at)
		VALUES ($1, 'raw@example.com', 'Raw Hit', now(), now())
	`, ownerID); err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	if _, err := env.Pool.Exec(context.Background(), `
		INSERT INTO chips (id, chip_id_raw, chip_id_normalized, manufacturer_hint, created_at)
		VALUES ($1, $2, $3, 'HomeAgain', now())
	`, chipID, "000000123456789", "985000000000001"); err != nil {
		t.Fatalf("insert chip: %v", err)
	}
	if _, err := env.Pool.Exec(context.Background(), `
		INSERT INTO pets (id, owner_id, chip_id, pet_name, species, active, registered_at, updated_at)
		VALUES ($1, $2, $3, 'Scout', 'dog', true, now(), now())
	`, petID, ownerID, chipID); err != nil {
		t.Fatalf("insert pet: %v", err)
	}

	results, _, err := s.LookupRegistrations(context.Background(), "000000123456789")
	if err != nil {
		t.Fatalf("LookupRegistrations returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(results))
	}
}

func TestLookupRegistrationsHitsChipIDNormalizedWhenRawDiffers(t *testing.T) {
	env := testutil.StartPostgres(t)
	s := store.New(env.Pool, 5*time.Second)

	ownerID := uuid.NewString()
	chipID := uuid.NewString()
	petID := uuid.NewString()
	if _, err := env.Pool.Exec(context.Background(), `
		INSERT INTO owners (id, email, name, created_at, updated_at)
		VALUES ($1, 'normalized@example.com', 'Normalized Hit', now(), now())
	`, ownerID); err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	if _, err := env.Pool.Exec(context.Background(), `
		INSERT INTO chips (id, chip_id_raw, chip_id_normalized, manufacturer_hint, created_at)
		VALUES ($1, $2, $3, 'Unknown manufacturer', now())
	`, chipID, "legacy-format", "000000123456789"); err != nil {
		t.Fatalf("insert chip: %v", err)
	}
	if _, err := env.Pool.Exec(context.Background(), `
		INSERT INTO pets (id, owner_id, chip_id, pet_name, species, active, registered_at, updated_at)
		VALUES ($1, $2, $3, 'Poppy', 'cat', true, now(), now())
	`, petID, ownerID, chipID); err != nil {
		t.Fatalf("insert pet: %v", err)
	}

	results, normalized, err := s.LookupRegistrations(context.Background(), "123456789")
	if err != nil {
		t.Fatalf("LookupRegistrations returned error: %v", err)
	}
	if normalized.Normalized != "000000123456789" {
		t.Fatalf("unexpected normalized value: %s", normalized.Normalized)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(results))
	}
	if results[0].PetName != "Poppy" {
		t.Fatalf("unexpected pet name: %s", results[0].PetName)
	}
}

func TestLookupRegistrationsAcceptsCanonicalizedHexLookup(t *testing.T) {
	env := testutil.StartPostgres(t)
	s := store.New(env.Pool, 5*time.Second)

	normalized, err := chip.Normalize("0A00000000")
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	ownerID := uuid.NewString()
	chipID := uuid.NewString()
	petID := uuid.NewString()
	if _, err := env.Pool.Exec(context.Background(), `
		INSERT INTO owners (id, email, name, created_at, updated_at)
		VALUES ($1, 'hex@example.com', 'Hex Lookup', now(), now())
	`, ownerID); err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	if _, err := env.Pool.Exec(context.Background(), `
		INSERT INTO chips (id, chip_id_raw, chip_id_normalized, manufacturer_hint, created_at)
		VALUES ($1, $2, $3, $4, now())
	`, chipID, normalized.Raw, normalized.Normalized, normalized.ManufacturerHint); err != nil {
		t.Fatalf("insert chip: %v", err)
	}
	if _, err := env.Pool.Exec(context.Background(), `
		INSERT INTO pets (id, owner_id, chip_id, pet_name, species, active, registered_at, updated_at)
		VALUES ($1, $2, $3, 'Hex Pet', 'dog', true, now(), now())
	`, petID, ownerID, chipID); err != nil {
		t.Fatalf("insert pet: %v", err)
	}

	results, _, err := s.LookupRegistrations(context.Background(), "0a-00 00-0000")
	if err != nil {
		t.Fatalf("LookupRegistrations returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(results))
	}
}
