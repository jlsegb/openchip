package app_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/openchip/openchip/api/internal/app"
	"github.com/openchip/openchip/api/internal/config"
	"github.com/openchip/openchip/api/internal/email"
	"github.com/openchip/openchip/api/internal/store"
	"github.com/openchip/openchip/api/internal/testutil"
)

type envelope[T any] struct {
	Data  T `json:"data"`
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type mockEmailSender struct {
	mu       sync.Mutex
	messages []email.Message
	ch       chan email.Message
}

func newMockEmailSender() *mockEmailSender {
	return &mockEmailSender{ch: make(chan email.Message, 64)}
}

func (m *mockEmailSender) Send(_ context.Context, msg email.Message) error {
	m.mu.Lock()
	m.messages = append(m.messages, msg)
	m.mu.Unlock()
	select {
	case m.ch <- msg:
	default:
	}
	return nil
}

func (m *mockEmailSender) waitForSubject(t *testing.T, contains string) email.Message {
	t.Helper()
	timeout := time.After(3 * time.Second)
	for {
		select {
		case msg := <-m.ch:
			if strings.Contains(msg.Subject, contains) {
				return msg
			}
		case <-timeout:
			t.Fatalf("timed out waiting for email containing subject %q", contains)
		}
	}
}

func newTestAPI(t *testing.T) (*httptest.Server, *pgxpool.Pool, *mockEmailSender) {
	t.Helper()

	env := testutil.StartPostgres(t)
	mailer := newMockEmailSender()
	_, trustedLoopback, _ := net.ParseCIDR("127.0.0.1/32")
	cfg := config.Config{
		Env:              "test",
		Port:             "0",
		DatabaseURL:      env.DSN,
		JWTSecret:        "0123456789abcdef0123456789abcdef",
		FromEmail:        "noreply@openchip.test",
		AdminEmail:       "admin@openchip.test",
		BaseURL:          "http://example.test",
		ShelterAPIKeys:   map[string]string{"valid-key": "Shelter Test"},
		TrustedProxyNets: []*net.IPNet{trustedLoopback},
		LookupRatePerMin: 60,
		AuthRatePerMin:   5,
		QueryTimeout:     5 * time.Second,
		MagicLinkTTL:     15 * time.Minute,
		JWTExpiry:        30 * 24 * time.Hour,
		TransferExpiry:   48 * time.Hour,
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := app.New(cfg, store.New(env.Pool, cfg.QueryTimeout), mailer, logger)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server, env.Pool, mailer
}

func TestFullRegistrationFlowMagicLinkVerifyCreatePet(t *testing.T) {
	server, _, mailer := newTestAPI(t)

	postNoResp(t, http.MethodPost, server, "/api/v1/auth/magic-link", "", map[string]string{
		"email": "owner@example.com",
		"name":  "Sam Owner",
	}, http.StatusOK, nil)
	token := mailer.waitForToken(t, "owner@example.com", "Click to sign in to OpenChip")
	verify := getJSON[map[string]string](t, server, "/api/v1/auth/verify?token="+token, "", http.StatusOK)
	jwt := verify["token"]
	if jwt == "" {
		t.Fatalf("expected jwt token")
	}

	pet := postJSON[map[string]any](t, server, "/api/v1/pets", jwt, map[string]any{
		"chip_id":   "123456789",
		"pet_name":  "Milo",
		"species":   "dog",
		"breed":     "Lab",
		"color":     "Black",
		"notes":     "Friendly",
		"photo_url": nil,
	}, http.StatusCreated, nil)

	if pet["pet_name"] != "Milo" {
		t.Fatalf("unexpected pet name: %#v", pet["pet_name"])
	}
	if pet["chip_id_normalized"] != "000000123456789" {
		t.Fatalf("unexpected normalized chip id: %#v", pet["chip_id_normalized"])
	}
}

func TestPublicLookupRegisteredChipReturnsPartialOwnerInfo(t *testing.T) {
	server, pool, mailer := newTestAPI(t)
	registerOwnerAndPet(t, server, pool, mailer, "lookup@example.com", "Luna Owner", "985000000000123", "Luna")

	resp := getJSON[map[string]any](t, server, "/api/v1/lookup/985000000000123", "", http.StatusOK)
	if resp["found"] != true {
		t.Fatalf("expected found lookup")
	}
	registrations := resp["registrations"].([]any)
	if len(registrations) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(registrations))
	}
	first := registrations[0].(map[string]any)
	if first["owner_first_name"] != "Luna" {
		t.Fatalf("unexpected owner_first_name: %#v", first["owner_first_name"])
	}
	if _, ok := first["owner_email"]; ok {
		t.Fatalf("public lookup should not expose owner email")
	}
	if _, ok := first["owner_phone"]; ok {
		t.Fatalf("public lookup should not expose owner phone")
	}
}

func TestPublicLookupUnregisteredChipReturnsManufacturerHintOnly(t *testing.T) {
	server, _, _ := newTestAPI(t)

	resp := getJSON[map[string]any](t, server, "/api/v1/lookup/985000000000999", "", http.StatusOK)
	if resp["found"] != false {
		t.Fatalf("expected found=false")
	}
	if resp["manufacturer_hint"] != "HomeAgain" {
		t.Fatalf("unexpected manufacturer hint: %#v", resp["manufacturer_hint"])
	}
	if len(resp["registrations"].([]any)) != 0 {
		t.Fatalf("expected empty registrations")
	}
}

func TestShelterLookupUsesMediatedContactWithValidAPIKey(t *testing.T) {
	server, pool, mailer := newTestAPI(t)
	registerOwnerAndPet(t, server, pool, mailer, "shelter@example.com", "Jamie Shelter", "982000000000321", "Pepper")

	resp := postJSON[map[string]any](t, server, "/api/v1/shelter/found", "", map[string]string{
		"chip_id":      "982000000000321",
		"organization": "Downtown Shelter",
		"location":     "Boston",
		"notes":        "Found near park",
	}, http.StatusOK, map[string]string{"X-API-Key": "valid-key"})

	if resp["found"] != true {
		t.Fatalf("expected found lookup")
	}
	registrations := resp["registrations"].([]any)
	first := registrations[0].(map[string]any)
	if _, ok := first["owner_email"]; ok {
		t.Fatalf("shelter lookup should not expose owner email")
	}
	if _, ok := first["owner_phone"]; ok {
		t.Fatalf("shelter lookup should not expose owner phone")
	}
	if first["contact_owner_via"] != "mediated_notification" {
		t.Fatalf("unexpected contact mode: %#v", first["contact_owner_via"])
	}
}

func TestShelterLookupRejectsInvalidAPIKey(t *testing.T) {
	server, _, _ := newTestAPI(t)

	body := mustJSON(t, map[string]string{"chip_id": "985000000000001", "organization": "Bad Key Shelter"})
	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/shelter/found", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "invalid-key")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer closeBody(t, resp)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestLookupTriggersOwnerNotificationEmail(t *testing.T) {
	server, pool, mailer := newTestAPI(t)
	registerOwnerAndPet(t, server, pool, mailer, "notify@example.com", "Nora Notify", "981000000000444", "Comet")

	getJSON[map[string]any](t, server, "/api/v1/lookup/981000000000444", "", http.StatusOK)
	msg := mailer.waitForSubject(t, "chip was just scanned")
	if msg.To != "notify@example.com" {
		t.Fatalf("unexpected email recipient: %s", msg.To)
	}
}

func TestTransferFlowInitiateConfirmDeactivatesOldPetAndCreatesNewPet(t *testing.T) {
	server, pool, mailer := newTestAPI(t)
	jwt := registerOwnerAndPet(t, server, pool, mailer, "current@example.com", "Casey Current", "900000000000123", "Rex")
	petID := fetchPetIDByName(t, pool, "Rex")

	resp := postJSON[map[string]any](t, server, "/api/v1/pets/"+petID+"/transfer", jwt, map[string]string{
		"to_email": "newowner@example.com",
		"note":     "Adoption complete",
	}, http.StatusCreated, nil)

	token := resp["token"].(string)
	postNoResp(t, http.MethodPost, server, "/api/v1/transfers/"+token+"/confirm", "", map[string]string{}, http.StatusOK, nil)

	activeCounts := map[string]int{}
	rows, err := pool.Query(context.Background(), `
		SELECT o.email, COUNT(*)
		FROM pets p
		JOIN owners o ON o.id = p.owner_id
		WHERE p.chip_id = (SELECT chip_id FROM pets WHERE id = $1)
		  AND p.active = true
		GROUP BY o.email
	`, petID)
	if err != nil {
		t.Fatalf("query active pets: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var email string
		var count int
		if err := rows.Scan(&email, &count); err != nil {
			t.Fatalf("scan row: %v", err)
		}
		activeCounts[email] = count
	}
	if activeCounts["newowner@example.com"] != 1 {
		t.Fatalf("expected new owner to have active pet, got %#v", activeCounts)
	}
	if activeCounts["current@example.com"] != 0 {
		t.Fatalf("expected current owner to have no active pet, got %#v", activeCounts)
	}
}

func TestTransferExpiryRejectsExpiredToken(t *testing.T) {
	server, pool, mailer := newTestAPI(t)
	jwt := registerOwnerAndPet(t, server, pool, mailer, "expires@example.com", "Erin Expires", "985000000000555", "Maple")
	petID := fetchPetIDByName(t, pool, "Maple")
	resp := postJSON[map[string]any](t, server, "/api/v1/pets/"+petID+"/transfer", jwt, map[string]string{
		"to_email": "later@example.com",
	}, http.StatusCreated, nil)
	token := resp["token"].(string)

	if _, err := pool.Exec(context.Background(), `UPDATE transfers SET expires_at = now() - interval '1 minute' WHERE token = $1`, token); err != nil {
		t.Fatalf("expire transfer: %v", err)
	}

	body := mustJSON(t, map[string]string{})
	req, err := http.NewRequest(http.MethodPost, server.URL+"/api/v1/transfers/"+token+"/confirm", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	respHTTP, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer closeBody(t, respHTTP)
	if respHTTP.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", respHTTP.StatusCode, http.StatusBadRequest)
	}
}

func TestRateLimitingLookupReturns429OnSixtyFirstRequest(t *testing.T) {
	server, _, _ := newTestAPI(t)

	for i := 1; i <= 61; i++ {
		req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v1/lookup/985000000000777", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("X-Forwarded-For", "203.0.113.10")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("do request %d: %v", i, err)
		}
		closeBody(t, resp)
		if i <= 60 && resp.StatusCode != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200", i, resp.StatusCode)
		}
		if i == 61 && resp.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("request 61 status = %d, want 429", resp.StatusCode)
		}
	}
}

func TestMagicLinkExpiryRejected(t *testing.T) {
	server, pool, _ := newTestAPI(t)
	ownerID := uuid.NewString()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO owners (id, email, name, created_at, updated_at)
		VALUES ($1, 'expired@example.com', 'Expired Owner', now(), now())
	`, ownerID); err != nil {
		t.Fatalf("insert owner: %v", err)
	}
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO magic_links (id, owner_id, token, expires_at, created_at)
		VALUES ($1, $2, $3, now() - interval '1 minute', now())
	`, uuid.NewString(), ownerID, hashTestToken("expired-token")); err != nil {
		t.Fatalf("insert magic link: %v", err)
	}

	resp := rawGet(t, server, "/api/v1/auth/verify?token=expired-token", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestMagicLinkSingleUseRejectedOnSecondAttempt(t *testing.T) {
	server, _, mailer := newTestAPI(t)

	postNoResp(t, http.MethodPost, server, "/api/v1/auth/magic-link", "", map[string]string{
		"email": "singleuse@example.com",
		"name":  "Single Use",
	}, http.StatusOK, nil)
	token := mailer.waitForToken(t, "singleuse@example.com", "Click to sign in to OpenChip")
	first := rawGet(t, server, "/api/v1/auth/verify?token="+token, "")
	if first.StatusCode != http.StatusOK {
		t.Fatalf("first verify status = %d, want 200", first.StatusCode)
	}
	closeBody(t, first)

	second := rawGet(t, server, "/api/v1/auth/verify?token="+token, "")
	if second.StatusCode != http.StatusUnauthorized {
		t.Fatalf("second verify status = %d, want 401", second.StatusCode)
	}
	closeBody(t, second)
}

func TestAccountDeletionAnonymizesOwnerAndPreservesChipAndLookupRecords(t *testing.T) {
	server, pool, mailer := newTestAPI(t)
	jwt := registerOwnerAndPet(t, server, pool, mailer, "delete@example.com", "Drew Delete", "985000000000888", "Olive")

	getJSON[map[string]any](t, server, "/api/v1/lookup/985000000000888", "", http.StatusOK)
	deleteNoResp(t, server, "/api/v1/account", jwt, map[string]string{}, http.StatusOK)

	var email, name string
	var activeCount, chipCount, lookupCount int
	if err := pool.QueryRow(context.Background(), `SELECT email, name FROM owners WHERE email LIKE 'deleted+%'`).Scan(&email, &name); err != nil {
		t.Fatalf("query anonymized owner: %v", err)
	}
	if !strings.HasPrefix(email, "deleted+") {
		t.Fatalf("unexpected anonymized email: %s", email)
	}
	if name != "Deleted Owner" {
		t.Fatalf("unexpected anonymized name: %s", name)
	}
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM pets WHERE active = true`).Scan(&activeCount); err != nil {
		t.Fatalf("count active pets: %v", err)
	}
	if activeCount != 0 {
		t.Fatalf("expected no active pets, got %d", activeCount)
	}
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM chips`).Scan(&chipCount); err != nil {
		t.Fatalf("count chips: %v", err)
	}
	if chipCount != 1 {
		t.Fatalf("expected chip record to remain, got %d", chipCount)
	}
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM lookups`).Scan(&lookupCount); err != nil {
		t.Fatalf("count lookups: %v", err)
	}
	if lookupCount != 1 {
		t.Fatalf("expected lookup record to remain, got %d", lookupCount)
	}
}

func registerOwnerAndPet(t *testing.T, server *httptest.Server, pool *pgxpool.Pool, mailer *mockEmailSender, emailAddress, ownerName, chipID, petName string) string {
	t.Helper()
	postNoResp(t, http.MethodPost, server, "/api/v1/auth/magic-link", "", map[string]string{
		"email": emailAddress,
		"name":  ownerName,
	}, http.StatusOK, nil)

	verifyResp := getJSON[map[string]string](t, server, "/api/v1/auth/verify?token="+mailer.waitForToken(t, emailAddress, "Click to sign in to OpenChip"), "", http.StatusOK)
	jwt := verifyResp["token"]
	postNoResp(t, http.MethodPost, server, "/api/v1/pets", jwt, map[string]any{
		"chip_id":  chipID,
		"pet_name": petName,
		"species":  "dog",
	}, http.StatusCreated, nil)
	return jwt
}

func fetchPetIDByName(t *testing.T, pool *pgxpool.Pool, petName string) string {
	t.Helper()
	var petID string
	if err := pool.QueryRow(context.Background(), `SELECT id::text FROM pets WHERE pet_name = $1 ORDER BY registered_at DESC LIMIT 1`, petName).Scan(&petID); err != nil {
		t.Fatalf("fetch pet id: %v", err)
	}
	return petID
}

func rawGet(t *testing.T, server *httptest.Server, path, jwt string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, server.URL+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if jwt != "" {
		req.Header.Set("Authorization", "Bearer "+jwt)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func getJSON[T any](t *testing.T, server *httptest.Server, path, jwt string, wantStatus int) T {
	t.Helper()
	resp := rawGet(t, server, path, jwt)
	defer closeBody(t, resp)
	if resp.StatusCode != wantStatus {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want %d, body=%s", resp.StatusCode, wantStatus, string(body))
	}
	return decodeEnvelope[T](t, resp.Body)
}

func postJSON[T any](t *testing.T, server *httptest.Server, path, jwt string, body any, wantStatus int, extraHeaders map[string]string) T {
	t.Helper()
	resp := doJSONRequest(t, http.MethodPost, server, path, jwt, body, wantStatus, extraHeaders)
	defer closeBody(t, resp)
	return decodeEnvelope[T](t, resp.Body)
}

func postNoResp(t *testing.T, method string, server *httptest.Server, path, jwt string, body any, wantStatus int, extraHeaders map[string]string) {
	t.Helper()
	resp := doJSONRequest(t, method, server, path, jwt, body, wantStatus, extraHeaders)
	defer closeBody(t, resp)
}

func deleteNoResp(t *testing.T, server *httptest.Server, path, jwt string, body any, wantStatus int) {
	t.Helper()
	resp := doJSONRequest(t, http.MethodDelete, server, path, jwt, body, wantStatus, nil)
	defer closeBody(t, resp)
}

func closeBody(t *testing.T, resp *http.Response) {
	t.Helper()
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("close response body: %v", err)
	}
}

func doJSONRequest(t *testing.T, method string, server *httptest.Server, path, jwt string, body any, wantStatus int, extraHeaders map[string]string) *http.Response {
	t.Helper()
	payload := mustJSON(t, body)
	req, err := http.NewRequest(method, server.URL+path, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if jwt != "" {
		req.Header.Set("Authorization", "Bearer "+jwt)
	}
	for key, value := range extraHeaders {
		req.Header.Set(key, value)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	if resp.StatusCode != wantStatus {
		raw, _ := io.ReadAll(resp.Body)
		closeBody(t, resp)
		t.Fatalf("status = %d, want %d, body=%s", resp.StatusCode, wantStatus, string(raw))
	}
	return resp
}

func mustJSON(t *testing.T, body any) []byte {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return payload
}

func decodeEnvelope[T any](t *testing.T, reader io.Reader) T {
	t.Helper()
	var payload envelope[T]
	if err := json.NewDecoder(reader).Decode(&payload); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if payload.Error != nil {
		t.Fatalf("unexpected api error: %s", payload.Error.Message)
	}
	return payload.Data
}

func (m *mockEmailSender) waitForToken(t *testing.T, to, subjectContains string) string {
	t.Helper()
	timeout := time.After(3 * time.Second)
	for {
		select {
		case msg := <-m.ch:
			if msg.To == to && strings.Contains(msg.Subject, subjectContains) {
				return extractTokenFromBody(t, msg.TextBody)
			}
		case <-timeout:
			t.Fatalf("timed out waiting for token email to %s", to)
		}
	}
}

func extractTokenFromBody(t *testing.T, body string) string {
	t.Helper()
	for _, part := range strings.Fields(body) {
		if strings.Contains(part, "token=") {
			parsed, err := url.Parse(strings.TrimSpace(part))
			if err != nil {
				t.Fatalf("parse token url: %v", err)
			}
			token := parsed.Query().Get("token")
			if token != "" {
				return token
			}
		}
	}
	t.Fatalf("token not found in email body")
	return ""
}

func hashTestToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
