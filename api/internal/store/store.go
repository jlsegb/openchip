package store

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openchip/openchip/api/internal/chip"
	"github.com/openchip/openchip/api/internal/model"
)

const (
	localReferenceNodeID = "22222222-2222-2222-2222-222222222222"
)

type Store struct {
	db      *pgxpool.Pool
	timeout time.Duration
}

type PetInput struct {
	ChipID      string     `json:"chip_id"`
	PetName     string     `json:"pet_name"`
	Species     string     `json:"species"`
	Breed       *string    `json:"breed"`
	Color       *string    `json:"color"`
	DateOfBirth *time.Time `json:"date_of_birth"`
	Notes       *string    `json:"notes"`
	PhotoURL    *string    `json:"photo_url"`
}

func New(db *pgxpool.Pool, timeout time.Duration) *Store {
	return &Store{db: db, timeout: timeout}
}

func (s *Store) ctx(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, s.timeout)
}

func (s *Store) Health(ctx context.Context) error {
	c, cancel := s.ctx(ctx)
	defer cancel()
	return s.db.Ping(c)
}

func (s *Store) FindOrCreateOwner(ctx context.Context, email, name string) (model.Owner, error) {
	c, cancel := s.ctx(ctx)
	defer cancel()

	email = strings.TrimSpace(strings.ToLower(email))
	var owner model.Owner
	query := `
		INSERT INTO owners (id, email, name, created_at, updated_at)
		VALUES ($1, lower($2), COALESCE(NULLIF($3, ''), split_part($2, '@', 1)), now(), now())
		ON CONFLICT (email) DO UPDATE SET
			name = COALESCE(NULLIF(EXCLUDED.name, ''), owners.name),
			updated_at = now()
		RETURNING id::text, email, name, phone, created_at, updated_at
	`
	err := s.db.QueryRow(c, query, uuid.New(), email, name).Scan(
		&owner.ID, &owner.Email, &owner.Name, &owner.Phone, &owner.CreatedAt, &owner.UpdatedAt,
	)
	if err != nil {
		return owner, err
	}
	if err := s.syncOwnerContact(c, owner.ID, owner.Email, owner.Phone, owner.CreatedAt, owner.UpdatedAt); err != nil {
		return owner, err
	}
	return owner, nil
}

func (s *Store) GetOwnerByID(ctx context.Context, ownerID string) (model.Owner, error) {
	c, cancel := s.ctx(ctx)
	defer cancel()
	var owner model.Owner
	err := s.db.QueryRow(c, `
		SELECT o.id::text, COALESCE(oc.email, o.email), o.name, COALESCE(oc.phone, o.phone), o.created_at, o.updated_at
		FROM owners o
		LEFT JOIN owner_contacts oc ON oc.owner_id = o.id
		WHERE o.id = $1
	`, ownerID).Scan(&owner.ID, &owner.Email, &owner.Name, &owner.Phone, &owner.CreatedAt, &owner.UpdatedAt)
	return owner, err
}

func (s *Store) UpdateOwnerProfile(ctx context.Context, ownerID, name string, phone *string) (model.Owner, error) {
	c, cancel := s.ctx(ctx)
	defer cancel()
	var owner model.Owner
	err := s.db.QueryRow(c, `
		UPDATE owners
		SET name = COALESCE(NULLIF($2, ''), name),
			phone = $3,
			updated_at = now()
		WHERE id = $1
		RETURNING id::text, email, name, phone, created_at, updated_at
	`, ownerID, name, phone).Scan(&owner.ID, &owner.Email, &owner.Name, &owner.Phone, &owner.CreatedAt, &owner.UpdatedAt)
	if err != nil {
		return owner, err
	}
	if err := s.syncOwnerContact(c, owner.ID, owner.Email, owner.Phone, owner.CreatedAt, owner.UpdatedAt); err != nil {
		return owner, err
	}
	return owner, s.appendEvent(c, "owner", owner.ID, "owner_profile_updated", map[string]interface{}{
		"name":  owner.Name,
		"phone": owner.Phone,
	}, "owner", ownerID, nil)
}

func (s *Store) CreateMagicLink(ctx context.Context, ownerID, token string, expiresAt time.Time) error {
	c, cancel := s.ctx(ctx)
	defer cancel()
	hashed := hashToken(token)
	_, err := s.db.Exec(c, `
		INSERT INTO magic_links (id, owner_id, token, expires_at, created_at)
		VALUES ($1, $2, $3, $4, now())
	`, uuid.New(), ownerID, hashed, expiresAt)
	return err
}

func (s *Store) ConsumeMagicLink(ctx context.Context, token string) (model.Owner, error) {
	c, cancel := s.ctx(ctx)
	defer cancel()
	tx, err := s.db.BeginTx(c, pgx.TxOptions{})
	if err != nil {
		return model.Owner{}, err
	}
	defer func() {
		_ = tx.Rollback(c)
	}()

	hashed := hashToken(token)
	var linkID, storedHash string
	var owner model.Owner
	err = tx.QueryRow(c, `
		SELECT ml.id::text, ml.token, o.id::text, o.email, o.name, o.phone, o.created_at, o.updated_at
		FROM magic_links ml
		JOIN owners o ON o.id = ml.owner_id
		WHERE ml.token = $1
			AND ml.used_at IS NULL
			AND ml.expires_at > now()
		FOR UPDATE
	`, hashed).Scan(&linkID, &storedHash, &owner.ID, &owner.Email, &owner.Name, &owner.Phone, &owner.CreatedAt, &owner.UpdatedAt)
	if err != nil {
		return model.Owner{}, err
	}
	if subtle.ConstantTimeCompare([]byte(storedHash), []byte(hashed)) != 1 {
		return model.Owner{}, pgx.ErrNoRows
	}
	if _, err := tx.Exec(c, `UPDATE magic_links SET used_at = now() WHERE id = $1`, linkID); err != nil {
		return model.Owner{}, err
	}
	return owner, tx.Commit(c)
}

func (s *Store) UpsertChip(ctx context.Context, normalized chip.Normalized) (string, string, string, error) {
	c, cancel := s.ctx(ctx)
	defer cancel()
	var chipPK, raw, iso string
	err := s.db.QueryRow(c, `
		INSERT INTO chips (id, chip_id_raw, chip_id_normalized, manufacturer_hint, created_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (chip_id_normalized) DO UPDATE SET
			chip_id_raw = EXCLUDED.chip_id_raw,
			manufacturer_hint = EXCLUDED.manufacturer_hint
		RETURNING id::text, chip_id_raw, chip_id_normalized
	`, uuid.New(), normalized.Raw, normalized.Normalized, normalized.ManufacturerHint).Scan(&chipPK, &raw, &iso)
	return chipPK, raw, iso, err
}

func (s *Store) CreatePet(ctx context.Context, ownerID string, input PetInput) (model.Pet, error) {
	norm, err := chip.Normalize(input.ChipID)
	if err != nil {
		return model.Pet{}, err
	}
	chipPK, _, _, err := s.UpsertChip(ctx, norm)
	if err != nil {
		return model.Pet{}, err
	}

	c, cancel := s.ctx(ctx)
	defer cancel()
	var pet model.Pet
	err = s.db.QueryRow(c, `
		INSERT INTO pets (
			id, owner_id, chip_id, pet_name, species, breed, color, date_of_birth, notes, photo_url,
			active, registered_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, true, now(), now()
		)
		RETURNING
			id::text, owner_id::text, chip_id::text, $11, $12, $13, pet_name, species, breed, color,
			date_of_birth, notes, photo_url, active, registered_at, updated_at
	`, uuid.New(), ownerID, chipPK, input.PetName, strings.ToLower(input.Species), input.Breed,
		input.Color, input.DateOfBirth, input.Notes, input.PhotoURL, norm.Raw, norm.Normalized, norm.ManufacturerHint,
	).Scan(
		&pet.ID, &pet.OwnerID, &pet.ChipPK, &pet.ChipIDRaw, &pet.ChipNormalized, &pet.Manufacturer,
		&pet.PetName, &pet.Species, &pet.Breed, &pet.Color, &pet.DateOfBirth, &pet.Notes, &pet.PhotoURL,
		&pet.Active, &pet.RegisteredAt, &pet.UpdatedAt,
	)
	if err != nil {
		return pet, err
	}
	if err := s.syncPetProjection(c, pet); err != nil {
		return pet, err
	}
	if err := s.appendEvent(c, "chip", pet.ChipPK, "registration_claim_created", map[string]interface{}{
		"owner_id":              pet.OwnerID,
		"pet_profile_id":        pet.ID,
		"chip_id_normalized":    pet.ChipNormalized,
		"manufacturer_hint":     pet.Manufacturer,
		"public_contact_policy": "mediated",
		"pet_name":              pet.PetName,
		"species":               pet.Species,
		"breed":                 pet.Breed,
		"color":                 pet.Color,
	}, "owner", ownerID, nil); err != nil {
		return pet, err
	}
	return pet, nil
}

func (s *Store) ListPets(ctx context.Context, ownerID string, activeOnly bool) ([]model.Pet, error) {
	c, cancel := s.ctx(ctx)
	defer cancel()
	query := `
		SELECT
			p.id::text, p.owner_id::text, c.id::text, c.chip_id_raw, c.chip_id_normalized, c.manufacturer_hint,
			p.pet_name, p.species, p.breed, p.color, p.date_of_birth, p.notes, p.photo_url,
			p.active, p.registered_at, p.updated_at
		FROM pets p
		JOIN chips c ON c.id = p.chip_id
		WHERE p.owner_id = $1`
	if activeOnly {
		query += ` AND p.active = true`
	}
	query += ` ORDER BY p.registered_at DESC`

	rows, err := s.db.Query(c, query, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pets []model.Pet
	for rows.Next() {
		var pet model.Pet
		if err := rows.Scan(
			&pet.ID, &pet.OwnerID, &pet.ChipPK, &pet.ChipIDRaw, &pet.ChipNormalized, &pet.Manufacturer,
			&pet.PetName, &pet.Species, &pet.Breed, &pet.Color, &pet.DateOfBirth, &pet.Notes, &pet.PhotoURL,
			&pet.Active, &pet.RegisteredAt, &pet.UpdatedAt,
		); err != nil {
			return nil, err
		}
		pets = append(pets, pet)
	}
	return pets, rows.Err()
}

func (s *Store) GetPet(ctx context.Context, ownerID, petID string) (model.Pet, error) {
	c, cancel := s.ctx(ctx)
	defer cancel()
	var pet model.Pet
	err := s.db.QueryRow(c, `
		SELECT
			p.id::text, p.owner_id::text, c.id::text, c.chip_id_raw, c.chip_id_normalized, c.manufacturer_hint,
			p.pet_name, p.species, p.breed, p.color, p.date_of_birth, p.notes, p.photo_url,
			p.active, p.registered_at, p.updated_at
		FROM pets p
		JOIN chips c ON c.id = p.chip_id
		WHERE p.id = $1 AND p.owner_id = $2
	`, petID, ownerID).Scan(
		&pet.ID, &pet.OwnerID, &pet.ChipPK, &pet.ChipIDRaw, &pet.ChipNormalized, &pet.Manufacturer,
		&pet.PetName, &pet.Species, &pet.Breed, &pet.Color, &pet.DateOfBirth, &pet.Notes, &pet.PhotoURL,
		&pet.Active, &pet.RegisteredAt, &pet.UpdatedAt,
	)
	return pet, err
}

func (s *Store) UpdatePet(ctx context.Context, ownerID, petID string, input PetInput) (model.Pet, error) {
	c, cancel := s.ctx(ctx)
	defer cancel()
	var pet model.Pet
	err := s.db.QueryRow(c, `
		UPDATE pets p
		SET pet_name = $3, species = $4, breed = $5, color = $6, date_of_birth = $7, notes = $8, photo_url = $9, updated_at = now()
		FROM chips c
		WHERE p.id = $1 AND p.owner_id = $2 AND c.id = p.chip_id
		RETURNING
			p.id::text, p.owner_id::text, c.id::text, c.chip_id_raw, c.chip_id_normalized, c.manufacturer_hint,
			p.pet_name, p.species, p.breed, p.color, p.date_of_birth, p.notes, p.photo_url,
			p.active, p.registered_at, p.updated_at
	`, petID, ownerID, input.PetName, strings.ToLower(input.Species), input.Breed, input.Color, input.DateOfBirth,
		input.Notes, input.PhotoURL,
	).Scan(
		&pet.ID, &pet.OwnerID, &pet.ChipPK, &pet.ChipIDRaw, &pet.ChipNormalized, &pet.Manufacturer,
		&pet.PetName, &pet.Species, &pet.Breed, &pet.Color, &pet.DateOfBirth, &pet.Notes, &pet.PhotoURL,
		&pet.Active, &pet.RegisteredAt, &pet.UpdatedAt,
	)
	if err != nil {
		return pet, err
	}
	if err := s.syncPetProjection(c, pet); err != nil {
		return pet, err
	}
	if err := s.appendEvent(c, "pet_profile", pet.ID, "pet_profile_updated", map[string]interface{}{
		"pet_name":   pet.PetName,
		"species":    pet.Species,
		"breed":      pet.Breed,
		"color":      pet.Color,
		"notes":      pet.Notes,
		"photo_url":  pet.PhotoURL,
		"owner_id":   pet.OwnerID,
		"chip_id":    pet.ChipPK,
		"chip_value": pet.ChipNormalized,
	}, "owner", ownerID, nil); err != nil {
		return pet, err
	}
	return pet, nil
}

func (s *Store) SoftDeletePet(ctx context.Context, ownerID, petID string) error {
	c, cancel := s.ctx(ctx)
	defer cancel()
	var chipID string
	err := s.db.QueryRow(c, `
		UPDATE pets
		SET active = false, updated_at = now()
		WHERE id = $1 AND owner_id = $2
		RETURNING chip_id::text
	`, petID, ownerID).Scan(&chipID)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(c, `
		UPDATE registration_claims
		SET status = 'inactive', updated_at = now()
		WHERE pet_profile_id = $1 AND claimant_owner_id = $2 AND status = 'active'
	`, petID, ownerID); err != nil {
		return err
	}
	return s.appendEvent(c, "pet_profile", petID, "registration_claim_deactivated", map[string]interface{}{
		"owner_id": ownerID,
		"chip_id":  chipID,
	}, "owner", ownerID, nil)
}

func (s *Store) LookupRegistrations(ctx context.Context, rawChip string) ([]model.LookupRegistration, chip.Normalized, error) {
	norm, err := chip.Normalize(rawChip)
	if err != nil {
		return nil, chip.Normalized{}, err
	}
	c, cancel := s.ctx(ctx)
	defer cancel()
	rows, err := s.db.Query(c, `
		SELECT o.id::text, p.pet_name, p.species, p.breed, p.color, o.name, split_part(o.name, ' ', 1), oc.phone, oc.email, c.manufacturer_hint
		FROM chips c
		JOIN pets p ON p.chip_id = c.id
		JOIN owners o ON o.id = p.owner_id
		LEFT JOIN owner_contacts oc ON oc.owner_id = o.id
		WHERE p.active = true
			AND (c.chip_id_normalized = $1 OR c.chip_id_raw = $2)
		ORDER BY p.registered_at DESC
	`, norm.Normalized, norm.Raw)
	if err != nil {
		return nil, chip.Normalized{}, err
	}
	defer rows.Close()

	var results []model.LookupRegistration
	for rows.Next() {
		var item model.LookupRegistration
		if err := rows.Scan(&item.OwnerID, &item.PetName, &item.Species, &item.Breed, &item.Color, &item.OwnerName, &item.OwnerFirst, &item.OwnerPhone, &item.OwnerEmail, &item.Manufacturer); err != nil {
			return nil, chip.Normalized{}, err
		}
		results = append(results, item)
	}
	return results, norm, rows.Err()
}

func (s *Store) LogLookup(ctx context.Context, rawInput string, norm chip.Normalized, found bool, ip, agent string) error {
	c, cancel := s.ctx(ctx)
	defer cancel()
	id := uuid.NewString()
	_, err := s.db.Exec(c, `
		INSERT INTO lookups (
			id, chip_id_queried, chip_id_normalized, found, looked_up_by_ip, looked_up_by_agent, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, now())
	`, id, rawInput, norm.Normalized, found, ip, agent)
	if err != nil {
		return err
	}
	return s.appendEvent(c, "chip_lookup", id, "lookup_recorded", map[string]interface{}{
		"chip_id_queried":    rawInput,
		"chip_id_normalized": norm.Normalized,
		"found":              found,
		"looked_up_by_ip":    ip,
		"looked_up_by_agent": agent,
	}, "public", ip, nil)
}

func (s *Store) MarkLookupNotified(ctx context.Context, chipNormalized string) error {
	c, cancel := s.ctx(ctx)
	defer cancel()
	_, err := s.db.Exec(c, `
		UPDATE lookups SET notified_owner_at = now()
		WHERE chip_id_normalized = $1 AND notified_owner_at IS NULL
	`, chipNormalized)
	return err
}

func (s *Store) CreateTransfer(ctx context.Context, chipPK, fromOwnerID, toEmail, initiatedBy, note string, expiresAt time.Time) (string, string, error) {
	toOwner, err := s.FindOrCreateOwner(ctx, toEmail, "")
	if err != nil {
		return "", "", err
	}
	token := strings.ReplaceAll(uuid.NewString(), "-", "")
	c, cancel := s.ctx(ctx)
	defer cancel()
	transferID := uuid.NewString()
	_, err = s.db.Exec(c, `
		INSERT INTO transfers (
			id, chip_id, from_owner_id, to_owner_id, initiated_by, initiator_note, status, token, expires_at, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, 'pending', $7, $8, now())
	`, transferID, chipPK, nullable(fromOwnerID), toOwner.ID, initiatedBy, note, token, expiresAt)
	if err != nil {
		return token, toOwner.ID, err
	}
	err = s.appendEvent(c, "chip", chipPK, "ownership_transfer_initiated", map[string]interface{}{
		"transfer_id":    transferID,
		"from_owner_id":  fromOwnerID,
		"to_owner_id":    toOwner.ID,
		"initiated_by":   initiatedBy,
		"initiator_note": note,
		"expires_at":     expiresAt,
		"target_contact": toEmail,
		"source_node_id": localReferenceNodeID,
	}, "owner", fromOwnerID, nil)
	return token, toOwner.ID, err
}

func nullable(value string) interface{} {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Store) ResolveTransfer(ctx context.Context, token, status string) error {
	c, cancel := s.ctx(ctx)
	defer cancel()
	var transferID, chipID string
	err := s.db.QueryRow(c, `
		UPDATE transfers
		SET status = $2, resolved_at = now()
		WHERE token = $1 AND status = 'pending'
		RETURNING id::text, chip_id::text
	`, token, status).Scan(&transferID, &chipID)
	if err != nil {
		return err
	}
	return s.appendEvent(c, "chip", chipID, "ownership_transfer_"+status, map[string]interface{}{
		"transfer_id": transferID,
		"token":       token,
	}, "system", localReferenceNodeID, nil)
}

func (s *Store) ApproveTransfer(ctx context.Context, token string) error {
	c, cancel := s.ctx(ctx)
	defer cancel()
	tx, err := s.db.BeginTx(c, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(c)
	}()

	var transferID, chipPK, fromOwnerID, toOwnerID string
	err = tx.QueryRow(c, `
		SELECT id::text, chip_id::text, COALESCE(from_owner_id::text, ''), to_owner_id::text
		FROM transfers
		WHERE token = $1 AND status = 'pending' AND expires_at > now()
	`, token).Scan(&transferID, &chipPK, &fromOwnerID, &toOwnerID)
	if err != nil {
		return err
	}

	if fromOwnerID != "" {
		if _, err := tx.Exec(c, `UPDATE pets SET active = false, updated_at = now() WHERE chip_id = $1 AND owner_id = $2 AND active = true`, chipPK, fromOwnerID); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(c, `
		INSERT INTO pets (id, owner_id, chip_id, pet_name, species, breed, color, date_of_birth, notes, photo_url, active, registered_at, updated_at)
		SELECT $1, $2, chip_id, pet_name, species, breed, color, date_of_birth, notes, photo_url, true, now(), now()
		FROM pets
		WHERE chip_id = $3
		ORDER BY registered_at DESC
		LIMIT 1
	`, uuid.New(), toOwnerID, chipPK); err != nil {
		return err
	}

	if _, err := tx.Exec(c, `UPDATE transfers SET status = 'approved', resolved_at = now() WHERE id = $1`, transferID); err != nil {
		return err
	}
	if _, err := tx.Exec(c, `
		UPDATE registration_claims
		SET status = 'transferred', updated_at = now()
		WHERE chip_id = $1 AND claimant_owner_id = $2 AND status = 'active'
	`, chipPK, nullable(fromOwnerID)); err != nil && fromOwnerID != "" {
		return err
	}
	if _, err := tx.Exec(c, `
		INSERT INTO registration_claims (id, chip_id, pet_profile_id, claimant_owner_id, source_node_id, status, claim_scope, created_at, updated_at)
		SELECT $1, p.chip_id, p.id, p.owner_id, $2, 'active', 'ownership', now(), now()
		FROM pets p
		WHERE p.chip_id = $3 AND p.owner_id = $4
		ORDER BY p.registered_at DESC
		LIMIT 1
	`, uuid.New(), localReferenceNodeID, chipPK, toOwnerID); err != nil {
		return err
	}
	if err := s.appendEvent(c, "chip", chipPK, "ownership_transferred", map[string]interface{}{
		"transfer_id":   transferID,
		"from_owner_id": fromOwnerID,
		"to_owner_id":   toOwnerID,
	}, "system", localReferenceNodeID, tx); err != nil {
		return err
	}
	return tx.Commit(c)
}

func (s *Store) ExportOwnerData(ctx context.Context, ownerID string) (map[string]interface{}, error) {
	owner, err := s.GetOwnerByID(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	pets, err := s.ListPets(ctx, ownerID, false)
	if err != nil {
		return nil, err
	}

	c, cancel := s.ctx(ctx)
	defer cancel()
	rows, err := s.db.Query(c, `
		SELECT l.id::text, l.chip_id_queried, l.chip_id_normalized, l.found, l.looked_up_by_ip, l.looked_up_by_agent, l.notified_owner_at, l.created_at
		FROM lookups l
		WHERE EXISTS (
			SELECT 1
			FROM pets p
			JOIN chips c ON c.id = p.chip_id
			WHERE p.owner_id = $1 AND c.chip_id_normalized = l.chip_id_normalized
		)
		ORDER BY l.created_at DESC
	`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lookups []map[string]interface{}
	for rows.Next() {
		var id, queried, normalized string
		var found bool
		var byIP, byAgent *string
		var notifiedAt *time.Time
		var createdAt time.Time
		if err := rows.Scan(&id, &queried, &normalized, &found, &byIP, &byAgent, &notifiedAt, &createdAt); err != nil {
			return nil, err
		}
		lookups = append(lookups, map[string]interface{}{
			"id":                 id,
			"chip_id_queried":    queried,
			"chip_id_normalized": normalized,
			"found":              found,
			"looked_up_by_ip":    byIP,
			"looked_up_by_agent": byAgent,
			"notified_owner_at":  notifiedAt,
			"created_at":         createdAt,
		})
	}

	private := map[string]interface{}{
		"owner_contact": map[string]interface{}{
			"email": owner.Email,
			"phone": owner.Phone,
		},
	}
	public := map[string]interface{}{
		"owner_profile": map[string]interface{}{
			"id":         owner.ID,
			"name":       owner.Name,
			"created_at": owner.CreatedAt,
			"updated_at": owner.UpdatedAt,
		},
		"pets":    pets,
		"lookups": lookups,
	}
	payload := map[string]interface{}{
		"profile": owner,
		"pets":    pets,
		"lookups": lookups,
		"public":  public,
		"private": private,
	}
	_ = s.recordExportBatch(c, "owner_export", ownerID, payload)
	return payload, rows.Err()
}

func (s *Store) AnonymizeOwner(ctx context.Context, ownerID string) error {
	c, cancel := s.ctx(ctx)
	defer cancel()
	tx, err := s.db.BeginTx(c, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(c)
	}()

	var email string
	if err := tx.QueryRow(c, `SELECT email FROM owners WHERE id = $1`, ownerID).Scan(&email); err != nil {
		return err
	}
	hash := sha256.Sum256([]byte(strings.ToLower(email)))
	replacement := fmt.Sprintf("deleted+%s@anonymized.openchip", hex.EncodeToString(hash[:8]))

	if _, err := tx.Exec(c, `UPDATE owners SET email = $2, name = 'Deleted Owner', phone = NULL, updated_at = now() WHERE id = $1`, ownerID, replacement); err != nil {
		return err
	}
	if _, err := tx.Exec(c, `
		INSERT INTO owner_contacts (owner_id, email, phone, created_at, updated_at)
		VALUES ($1, $2, NULL, now(), now())
		ON CONFLICT (owner_id) DO UPDATE SET email = EXCLUDED.email, phone = EXCLUDED.phone, updated_at = now()
	`, ownerID, replacement); err != nil {
		return err
	}
	if _, err := tx.Exec(c, `UPDATE pets SET active = false, updated_at = now() WHERE owner_id = $1`, ownerID); err != nil {
		return err
	}
	if _, err := tx.Exec(c, `UPDATE registration_claims SET status = 'inactive', updated_at = now() WHERE claimant_owner_id = $1 AND status = 'active'`, ownerID); err != nil {
		return err
	}
	if err := s.appendEvent(c, "owner", ownerID, "owner_anonymized", map[string]interface{}{
		"replacement_contact": replacement,
	}, "owner", ownerID, tx); err != nil {
		return err
	}
	return tx.Commit(c)
}

func (s *Store) CreateDispute(ctx context.Context, chipID, reporterName, reporterEmail, description string) error {
	norm, err := chip.Normalize(chipID)
	if err != nil {
		return err
	}
	chipPK, _, _, err := s.UpsertChip(ctx, norm)
	if err != nil {
		return err
	}
	c, cancel := s.ctx(ctx)
	defer cancel()
	disputeID := uuid.NewString()
	_, err = s.db.Exec(c, `
		INSERT INTO disputes (id, chip_id, reporter_email, reporter_name, description, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'open', now(), now())
	`, disputeID, chipPK, reporterEmail, reporterName, description)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(c, `
		UPDATE registration_claims SET status = 'disputed', updated_at = now()
		WHERE chip_id = $1 AND status = 'active'
	`, chipPK); err != nil {
		return err
	}
	return s.appendEvent(c, "chip", chipPK, "dispute_opened", map[string]interface{}{
		"dispute_id":      disputeID,
		"reporter_name":   reporterName,
		"reporter_email":  reporterEmail,
		"description":     description,
		"chip_id_raw":     chipID,
		"chip_normalized": norm.Normalized,
	}, "reporter", reporterEmail, nil)
}

func (s *Store) ListDisputes(ctx context.Context) ([]map[string]interface{}, error) {
	c, cancel := s.ctx(ctx)
	defer cancel()
	rows, err := s.db.Query(c, `
		SELECT d.id::text, c.chip_id_normalized, d.reporter_email, d.reporter_name, d.description, d.status, d.created_at, d.updated_at
		FROM disputes d
		JOIN chips c ON c.id = d.chip_id
		ORDER BY d.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []map[string]interface{}
	for rows.Next() {
		var id, chipID, reporterEmail, reporterName, description, status string
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &chipID, &reporterEmail, &reporterName, &description, &status, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		results = append(results, map[string]interface{}{
			"id":             id,
			"chip_id":        chipID,
			"reporter_email": reporterEmail,
			"reporter_name":  reporterName,
			"description":    description,
			"status":         status,
			"created_at":     createdAt,
			"updated_at":     updatedAt,
		})
	}
	return results, rows.Err()
}

func (s *Store) GetDispute(ctx context.Context, id string) (map[string]interface{}, error) {
	c, cancel := s.ctx(ctx)
	defer cancel()
	var result = map[string]interface{}{}
	var disputeID, chipID, reporterEmail, reporterName, description, status string
	var createdAt, updatedAt time.Time
	err := s.db.QueryRow(c, `
		SELECT d.id::text, c.chip_id_normalized, d.reporter_email, d.reporter_name, d.description, d.status, d.created_at, d.updated_at
		FROM disputes d
		JOIN chips c ON c.id = d.chip_id
		WHERE d.id = $1
	`, id).Scan(&disputeID, &chipID, &reporterEmail, &reporterName, &description, &status, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	result["id"] = disputeID
	result["chip_id"] = chipID
	result["reporter_email"] = reporterEmail
	result["reporter_name"] = reporterName
	result["description"] = description
	result["status"] = status
	result["created_at"] = createdAt
	result["updated_at"] = updatedAt
	return result, nil
}

func (s *Store) UpdateDispute(ctx context.Context, id, status, resolutionNote string) error {
	c, cancel := s.ctx(ctx)
	defer cancel()
	var chipID string
	err := s.db.QueryRow(c, `
		UPDATE disputes SET status = $2, description = CASE WHEN $3 = '' THEN description ELSE description || E'\n\nResolution: ' || $3 END, updated_at = now()
		WHERE id = $1
		RETURNING chip_id::text
	`, id, status, resolutionNote).Scan(&chipID)
	if err != nil {
		return err
	}
	if status == "resolved" {
		if _, err := s.db.Exec(c, `
			UPDATE registration_claims SET status = 'active', updated_at = now()
			WHERE chip_id = $1 AND status = 'disputed'
		`, chipID); err != nil {
			return err
		}
	}
	return s.appendEvent(c, "chip", chipID, "dispute_"+status, map[string]interface{}{
		"dispute_id":      id,
		"resolution_note": resolutionNote,
	}, "admin", localReferenceNodeID, nil)
}

func (s *Store) Stats(ctx context.Context) (map[string]interface{}, error) {
	c, cancel := s.ctx(ctx)
	defer cancel()
	stats := map[string]interface{}{}
	var totalChips int
	if err := s.db.QueryRow(c, `SELECT COUNT(*) FROM chips`).Scan(&totalChips); err != nil {
		return nil, err
	}
	var totalRegistrations int
	if err := s.db.QueryRow(c, `SELECT COUNT(*) FROM pets WHERE active = true`).Scan(&totalRegistrations); err != nil {
		return nil, err
	}
	var lookupsLast30 int
	if err := s.db.QueryRow(c, `SELECT COUNT(*) FROM lookups WHERE created_at >= now() - interval '30 days'`).Scan(&lookupsLast30); err != nil {
		return nil, err
	}
	var foundRate float64
	if err := s.db.QueryRow(c, `
		SELECT COALESCE(AVG(CASE WHEN found THEN 1.0 ELSE 0.0 END), 0)
		FROM lookups WHERE created_at >= now() - interval '30 days'
	`).Scan(&foundRate); err != nil {
		return nil, err
	}
	stats["total_chips"] = totalChips
	stats["total_registrations"] = totalRegistrations
	stats["lookups_last_30_days"] = lookupsLast30
	stats["found_rate"] = foundRate

	rows, err := s.db.Query(c, `
		SELECT manufacturer_hint, COUNT(*) AS total
		FROM chips
		GROUP BY manufacturer_hint
		ORDER BY total DESC
		LIMIT 5
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var top []map[string]interface{}
	for rows.Next() {
		var hint string
		var total int
		if err := rows.Scan(&hint, &total); err != nil {
			return nil, err
		}
		top = append(top, map[string]interface{}{"manufacturer_hint": hint, "count": total})
	}
	stats["top_manufacturer_hints"] = top
	return stats, rows.Err()
}

func (s *Store) LookupHistoryForPet(ctx context.Context, ownerID, petID string) ([]map[string]interface{}, error) {
	c, cancel := s.ctx(ctx)
	defer cancel()
	rows, err := s.db.Query(c, `
		SELECT l.id::text, l.chip_id_queried, l.chip_id_normalized, l.found, l.looked_up_by_ip, l.looked_up_by_agent, l.notified_owner_at, l.created_at
		FROM lookups l
		WHERE l.chip_id_normalized = (
			SELECT c.chip_id_normalized
			FROM pets p
			JOIN chips c ON c.id = p.chip_id
			WHERE p.id = $1 AND p.owner_id = $2
		)
		ORDER BY l.created_at DESC
	`, petID, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var history []map[string]interface{}
	for rows.Next() {
		var id, queried, normalized string
		var found bool
		var byIP, byAgent *string
		var notifiedAt *time.Time
		var createdAt time.Time
		if err := rows.Scan(&id, &queried, &normalized, &found, &byIP, &byAgent, &notifiedAt, &createdAt); err != nil {
			return nil, err
		}
		history = append(history, map[string]interface{}{
			"id":                 id,
			"chip_id_queried":    queried,
			"chip_id_normalized": normalized,
			"found":              found,
			"looked_up_by_ip":    byIP,
			"looked_up_by_agent": byAgent,
			"notified_owner_at":  notifiedAt,
			"created_at":         createdAt,
		})
	}
	return history, rows.Err()
}

func (s *Store) GetNodeMetadata(ctx context.Context) (map[string]interface{}, error) {
	c, cancel := s.ctx(ctx)
	defer cancel()
	return s.loadNodeMetadata(c)
}

func (s *Store) ExportPublicSnapshot(ctx context.Context) (map[string]interface{}, error) {
	c, cancel := s.ctx(ctx)
	defer cancel()

	node, err := s.loadNodeMetadata(c)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(c, `
		SELECT
			c.chip_id_normalized,
			c.manufacturer_hint,
			p.id::text,
			p.pet_name,
			p.species,
			p.breed,
			p.color,
			split_part(o.name, ' ', 1) AS owner_first_name
		FROM pets p
		JOIN chips c ON c.id = p.chip_id
		JOIN owners o ON o.id = p.owner_id
		WHERE p.active = true
		ORDER BY c.chip_id_normalized, p.registered_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var registrations []map[string]interface{}
	for rows.Next() {
		var chipID, manufacturer, petID, petName, species, ownerFirst string
		var breed, color *string
		if err := rows.Scan(&chipID, &manufacturer, &petID, &petName, &species, &breed, &color, &ownerFirst); err != nil {
			return nil, err
		}
		registrations = append(registrations, map[string]interface{}{
			"chip_id_normalized": chipID,
			"manufacturer_hint":  manufacturer,
			"pet_profile": map[string]interface{}{
				"id":       petID,
				"pet_name": petName,
				"species":  species,
				"breed":    breed,
				"color":    color,
			},
			"owner_first_name":   ownerFirst,
			"contact_visibility": "mediated",
		})
	}

	payload := map[string]interface{}{
		"format":               "openchip.public-snapshot.v1",
		"node":                 node,
		"public_registrations": registrations,
		"generated_at":         time.Now().UTC(),
	}
	if err := s.recordPublicSnapshot(c, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (s *Store) ExportEventStream(ctx context.Context, since *time.Time, limit int) (map[string]interface{}, error) {
	c, cancel := s.ctx(ctx)
	defer cancel()

	node, err := s.loadNodeMetadata(c)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 250
	}

	query := `
		SELECT id::text, aggregate_type, aggregate_id::text, event_type, payload_json, actor_type, actor_id, event_hash, previous_event_hash, signature, created_at
		FROM ownership_events
	`
	var rows pgx.Rows
	if since != nil {
		query += ` WHERE created_at >= $1 ORDER BY created_at ASC LIMIT $2`
		rows, err = s.db.Query(c, query, *since, limit)
	} else {
		query += ` ORDER BY created_at ASC LIMIT $1`
		rows, err = s.db.Query(c, query, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []map[string]interface{}
	for rows.Next() {
		var id, aggregateType, aggregateID, eventType, actorType, actorID, eventHash string
		var payload []byte
		var previousEventHash, signature *string
		var createdAt time.Time
		if err := rows.Scan(&id, &aggregateType, &aggregateID, &eventType, &payload, &actorType, &actorID, &eventHash, &previousEventHash, &signature, &createdAt); err != nil {
			return nil, err
		}
		var decoded map[string]interface{}
		if err := json.Unmarshal(payload, &decoded); err != nil {
			return nil, err
		}
		events = append(events, map[string]interface{}{
			"id":                  id,
			"aggregate_type":      aggregateType,
			"aggregate_id":        aggregateID,
			"event_type":          eventType,
			"payload":             decoded,
			"actor_type":          actorType,
			"actor_id":            actorID,
			"event_hash":          eventHash,
			"previous_event_hash": previousEventHash,
			"signature":           signature,
			"created_at":          createdAt,
		})
	}

	payload := map[string]interface{}{
		"format": "openchip.event-stream.v1",
		"node":   node,
		"events": events,
	}
	scope := "all"
	if since != nil {
		scope = "since:" + since.UTC().Format(time.RFC3339)
	}
	if err := s.recordExportBatch(c, "event_stream", scope, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (s *Store) syncOwnerContact(ctx context.Context, ownerID, email string, phone *string, createdAt, updatedAt time.Time) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO owner_contacts (owner_id, email, phone, created_at, updated_at)
		VALUES ($1, lower($2), $3, $4, $5)
		ON CONFLICT (owner_id) DO UPDATE
		SET email = EXCLUDED.email, phone = EXCLUDED.phone, updated_at = EXCLUDED.updated_at
	`, ownerID, email, phone, createdAt, updatedAt)
	return err
}

func (s *Store) syncPetProjection(ctx context.Context, pet model.Pet) error {
	if _, err := s.db.Exec(ctx, `
		INSERT INTO pet_profiles (id, chip_id, display_name, species, breed, color, date_of_birth, notes, photo_url, public_contact_policy, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'mediated', $10, $11)
		ON CONFLICT (id) DO UPDATE
		SET display_name = EXCLUDED.display_name,
			species = EXCLUDED.species,
			breed = EXCLUDED.breed,
			color = EXCLUDED.color,
			date_of_birth = EXCLUDED.date_of_birth,
			notes = EXCLUDED.notes,
			photo_url = EXCLUDED.photo_url,
			updated_at = EXCLUDED.updated_at
	`, pet.ID, pet.ChipPK, pet.PetName, pet.Species, pet.Breed, pet.Color, pet.DateOfBirth, pet.Notes, pet.PhotoURL, pet.RegisteredAt, pet.UpdatedAt); err != nil {
		return err
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO registration_claims (id, chip_id, pet_profile_id, claimant_owner_id, source_node_id, status, claim_scope, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'active', 'ownership', $6, $7)
		ON CONFLICT DO NOTHING
	`, uuid.New(), pet.ChipPK, pet.ID, pet.OwnerID, localReferenceNodeID, pet.RegisteredAt, pet.UpdatedAt)
	return err
}

func (s *Store) appendEvent(ctx context.Context, aggregateType, aggregateID, eventType string, payload map[string]interface{}, actorType, actorID string, tx pgx.Tx) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	createdAt := time.Now().UTC()

	var previousHash *string
	query := `
		SELECT event_hash
		FROM ownership_events
		WHERE aggregate_type = $1 AND aggregate_id = $2
		ORDER BY created_at DESC, id DESC
		LIMIT 1
	`
	if tx != nil {
		err = tx.QueryRow(ctx, query, aggregateType, aggregateID).Scan(&previousHash)
	} else {
		err = s.db.QueryRow(ctx, query, aggregateType, aggregateID).Scan(&previousHash)
	}
	if err != nil && err != pgx.ErrNoRows {
		return err
	}
	if err == pgx.ErrNoRows {
		previousHash = nil
	}

	hashMaterial := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s", aggregateType, aggregateID, eventType, string(payloadJSON), actorType, actorID, createdAt.Format(time.RFC3339Nano))
	if previousHash != nil {
		hashMaterial += "|" + *previousHash
	}
	sum := sha256.Sum256([]byte(hashMaterial))
	eventHash := hex.EncodeToString(sum[:])

	exec := func() error {
		_, err := s.db.Exec(ctx, `
			INSERT INTO ownership_events (
				id, aggregate_type, aggregate_id, event_type, payload_json, actor_type, actor_id, event_hash, previous_event_hash, signature, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULL, $10)
		`, uuid.New(), aggregateType, aggregateID, eventType, payloadJSON, actorType, actorID, eventHash, previousHash, createdAt)
		return err
	}
	if tx != nil {
		_, err = tx.Exec(ctx, `
			INSERT INTO ownership_events (
				id, aggregate_type, aggregate_id, event_type, payload_json, actor_type, actor_id, event_hash, previous_event_hash, signature, created_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULL, $10)
		`, uuid.New(), aggregateType, aggregateID, eventType, payloadJSON, actorType, actorID, eventHash, previousHash, createdAt)
		return err
	}
	return exec()
}

func (s *Store) loadNodeMetadata(ctx context.Context) (map[string]interface{}, error) {
	node := map[string]interface{}{}
	var id, orgSlug, orgName, slug, displayName, baseURL, status, federationMode string
	err := s.db.QueryRow(ctx, `
		SELECT n.id::text, o.slug, o.name, n.slug, n.display_name, n.public_base_url, n.status, n.federation_mode
		FROM nodes n
		JOIN organizations o ON o.id = n.organization_id
		ORDER BY n.created_at ASC
		LIMIT 1
	`).Scan(&id, &orgSlug, &orgName, &slug, &displayName, &baseURL, &status, &federationMode)
	if err != nil {
		return nil, err
	}
	node["id"] = id
	node["slug"] = slug
	node["display_name"] = displayName
	node["public_base_url"] = baseURL
	node["status"] = status
	node["federation_mode"] = federationMode
	node["organization"] = map[string]interface{}{
		"slug": orgSlug,
		"name": orgName,
	}
	node["protocol_version"] = "openchip-node/0.1"
	return node, nil
}

func (s *Store) recordPublicSnapshot(ctx context.Context, payload map[string]interface{}) error {
	node, ok := payload["node"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("snapshot missing node metadata")
	}
	nodeID, _ := node["id"].(string)
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(payloadJSON)
	_, err = s.db.Exec(ctx, `
		INSERT INTO public_snapshots (id, node_id, snapshot_type, payload_json, payload_hash, generated_at)
		VALUES ($1, $2, 'public_registry', $3, $4, now())
	`, uuid.New(), nodeID, payloadJSON, hex.EncodeToString(sum[:]))
	return err
}

func (s *Store) recordExportBatch(ctx context.Context, exportType, scope string, payload map[string]interface{}) error {
	node, err := s.loadNodeMetadata(ctx)
	if err != nil {
		return err
	}
	nodeID, _ := node["id"].(string)
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(payloadJSON)
	_, err = s.db.Exec(ctx, `
		INSERT INTO export_batches (id, node_id, export_type, scope, payload_json, payload_hash, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, now())
	`, uuid.New(), nodeID, exportType, scope, payloadJSON, hex.EncodeToString(sum[:]))
	return err
}
