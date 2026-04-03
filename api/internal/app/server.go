package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"

	"github.com/openchip/openchip/api/internal/auth"
	"github.com/openchip/openchip/api/internal/chip"
	"github.com/openchip/openchip/api/internal/config"
	"github.com/openchip/openchip/api/internal/email"
	"github.com/openchip/openchip/api/internal/httpx"
	"github.com/openchip/openchip/api/internal/middleware"
	"github.com/openchip/openchip/api/internal/model"
	"github.com/openchip/openchip/api/internal/store"
	"github.com/openchip/openchip/api/internal/validate"
)

type Server struct {
	cfg          config.Config
	store        *store.Store
	email        EmailSender
	logger       *slog.Logger
	authRateLock sync.Mutex
	authRates    map[string]rateWindow
}

type EmailSender interface {
	Send(context.Context, email.Message) error
}

type rateWindow struct {
	Count   int
	Expires time.Time
}

func New(cfg config.Config, store *store.Store, emailSvc EmailSender, logger *slog.Logger) http.Handler {
	s := &Server{
		cfg:       cfg,
		store:     store,
		email:     emailSvc,
		logger:    logger,
		authRates: map[string]rateWindow{},
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestLogger(logger))
	r.Use(chimiddleware.Recoverer)

	r.Get("/health", s.health)

	r.Route("/api/v1", func(r chi.Router) {
		publicIPLimit := func(limit int) func(http.Handler) http.Handler {
			return middleware.NewRateLimit(limit, func(req *http.Request) string {
				return middleware.ExtractClientIP(req, cfg.TrustedProxyNets)
			})
		}

		r.With(publicIPLimit(cfg.LookupRatePerMin)).Get("/lookup/{chip_id}", s.lookupPublic)
		r.Get("/lookup/{chip_id}/manufacturer", s.lookupManufacturer)
		r.With(publicIPLimit(10)).Post("/lookup/{chip_id}/contact", s.contactOwner)
		r.Post("/auth/magic-link", s.magicLink)
		r.With(publicIPLimit(10)).Get("/auth/verify", s.verifyMagicLink)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireJWT(cfg.JWTSecret, false))
			r.Get("/auth/me", s.me)
			r.Put("/auth/me", s.updateMe)
			r.Get("/pets", s.listPets)
			r.Post("/pets", s.createPet)
			r.Get("/pets/{pet_id}", s.getPet)
			r.Put("/pets/{pet_id}", s.updatePet)
			r.Delete("/pets/{pet_id}", s.deletePet)
			r.Get("/pets/{pet_id}/lookups", s.petLookupHistory)
			r.Post("/pets/{pet_id}/transfer", s.initiateTransfer)
			r.Get("/export", s.exportData)
			r.Delete("/account", s.deleteAccount)
		})

		r.Post("/transfers/{token}/confirm", s.confirmTransfer)
		r.Post("/transfers/{token}/reject", s.rejectTransfer)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAPIKey(cfg.ShelterAPIKeys))
			r.Post("/shelter/found", s.shelterFound)
		})
		r.With(publicIPLimit(5)).Post("/disputes", s.createDispute)
		r.With(publicIPLimit(cfg.LookupRatePerMin)).Get("/aaha/lookup/{chip_id}", s.aahaLookup)

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireJWT(cfg.JWTSecret, true))
			r.Get("/admin/disputes", s.adminListDisputes)
			r.Get("/admin/disputes/{id}", s.adminGetDispute)
			r.Put("/admin/disputes/{id}", s.adminUpdateDispute)
			r.Get("/admin/stats", s.adminStats)
		})
	})

	return r
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if err := s.store.Health(r.Context()); err != nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, "db_unhealthy", "database health check failed")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) lookupManufacturer(w http.ResponseWriter, r *http.Request) {
	if err := validate.ChipID(chi.URLParam(r, "chip_id")); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_chip_id", err.Error())
		return
	}
	norm, err := chip.Normalize(chi.URLParam(r, "chip_id"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_chip_id", err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"manufacturer_hint": norm.ManufacturerHint})
}

func (s *Server) lookupPublic(w http.ResponseWriter, r *http.Request) {
	chipID := chi.URLParam(r, "chip_id")
	if err := validate.ChipID(chipID); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_lookup", err.Error())
		return
	}
	results, norm, err := s.store.LookupRegistrations(r.Context(), chipID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_lookup", err.Error())
		return
	}

	agent := r.Header.Get("User-Agent")
	if err := s.store.LogLookup(r.Context(), chipID, norm, len(results) > 0, middleware.ExtractClientIP(r, s.cfg.TrustedProxyNets), agent); err != nil {
		middleware.LoggerFromContext(r.Context()).Warn("failed to log lookup",
			slog.String("request_id", middleware.RequestIDFromContext(r.Context())),
			slog.String("chip_id_normalized", norm.Normalized),
			slog.String("error", err.Error()),
		)
	}

	public := make([]map[string]interface{}, 0, len(results))
	for _, item := range results {
		public = append(public, map[string]interface{}{
			"pet_name":           item.PetName,
			"species":            item.Species,
			"breed":              item.Breed,
			"color":              item.Color,
			"owner_first_name":   item.OwnerFirst,
			"manufacturer_hint":  item.Manufacturer,
			"contact_owner_via":  "notification_request",
			"contact_visibility": "protected",
		})
	}

	if len(results) > 0 {
		go s.notifyOwners(r.Context(), results, norm.Normalized, agent)
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"found":             len(results) > 0,
		"manufacturer_hint": norm.ManufacturerHint,
		"registrations":     public,
	})
}

func (s *Server) shelterFound(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ChipID       string `json:"chip_id"`
		Organization string `json:"organization"`
		Location     string `json:"location"`
		Notes        string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "could not parse request body")
		return
	}
	if err := validate.ChipID(body.ChipID); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_lookup", err.Error())
		return
	}
	if err := validate.RequiredText("organization", body.Organization, 120); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_organization", err.Error())
		return
	}
	if err := validate.OptionalText("location", stringPtr(body.Location), 120); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_location", err.Error())
		return
	}
	if err := validate.OptionalText("notes", stringPtr(body.Notes), 2000); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_notes", err.Error())
		return
	}
	results, norm, err := s.store.LookupRegistrations(r.Context(), body.ChipID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_lookup", err.Error())
		return
	}
	agent := body.Organization
	if agent == "" {
		agent = middleware.APIOrgFromContext(r.Context())
	}
	if err := s.store.LogLookup(r.Context(), body.ChipID, norm, len(results) > 0, middleware.ExtractClientIP(r, s.cfg.TrustedProxyNets), agent); err != nil {
		middleware.LoggerFromContext(r.Context()).Warn("failed to log lookup",
			slog.String("request_id", middleware.RequestIDFromContext(r.Context())),
			slog.String("chip_id_normalized", norm.Normalized),
			slog.String("error", err.Error()),
		)
	}
	if len(results) > 0 {
		go s.notifyOwners(r.Context(), results, norm.Normalized, agent)
	}

	full := make([]map[string]interface{}, 0, len(results))
	for _, item := range results {
		full = append(full, map[string]interface{}{
			"pet_name":          item.PetName,
			"species":           item.Species,
			"breed":             item.Breed,
			"color":             item.Color,
			"owner_name":        item.OwnerName,
			"owner_phone":       item.OwnerPhone,
			"owner_email":       item.OwnerEmail,
			"manufacturer_hint": item.Manufacturer,
		})
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"found":             len(results) > 0,
		"manufacturer_hint": norm.ManufacturerHint,
		"registrations":     full,
	})
}

func (s *Server) contactOwner(w http.ResponseWriter, r *http.Request) {
	chipID := chi.URLParam(r, "chip_id")
	if err := validate.ChipID(chipID); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_lookup", err.Error())
		return
	}
	results, norm, err := s.store.LookupRegistrations(r.Context(), chipID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_lookup", err.Error())
		return
	}
	if len(results) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]bool{"sent": false})
		return
	}
	go s.notifyOwners(r.Context(), results, norm.Normalized, "OpenChip contact request")
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"sent": true})
}

func (s *Server) magicLink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "could not parse request body")
		return
	}
	body.Email = strings.TrimSpace(strings.ToLower(body.Email))
	if err := validate.Email(body.Email); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_email", err.Error())
		return
	}
	if body.Name != "" {
		if err := validate.OptionalText("name", &body.Name, 120); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_name", err.Error())
			return
		}
	}
	if !s.allowAuthEmail(body.Email) {
		httpx.WriteError(w, http.StatusTooManyRequests, "rate_limited", "too many auth attempts for this email")
		return
	}

	owner, err := s.store.FindOrCreateOwner(r.Context(), body.Email, body.Name)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "owner_error", "could not create owner")
		return
	}
	token, err := randomHex(32)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "token_error", "could not create token")
		return
	}
	if err := s.store.CreateMagicLink(r.Context(), owner.ID, token, time.Now().Add(s.cfg.MagicLinkTTL)); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "magic_link_error", "could not create magic link")
		return
	}

	link := fmt.Sprintf("%s/auth/verify?token=%s", s.cfg.BaseURL, token)
	subject, text, html := email.MagicLink(link)
	s.sendEmailAsync(r.Context(), email.Message{To: owner.Email, Subject: subject, TextBody: text, HTMLBody: html}, owner.ID, "")

	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (s *Server) verifyMagicLink(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_token", "token is required")
		return
	}
	owner, err := s.store.ConsumeMagicLink(r.Context(), token)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_token", "token is invalid or expired")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "verify_failed", "could not verify token")
		return
	}
	role := "owner"
	if strings.EqualFold(owner.Email, s.cfg.AdminEmail) {
		role = "admin"
	}
	jwtToken, err := auth.Sign(s.cfg.JWTSecret, owner.ID, owner.Email, role, s.cfg.JWTExpiry)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "jwt_error", "could not create session token")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"token": jwtToken, "role": role})
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	owner, err := s.store.GetOwnerByID(r.Context(), claims.Subject)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "owner not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, owner)
}

func (s *Server) updateMe(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	var body struct {
		Name  string  `json:"name"`
		Phone *string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "could not parse request body")
		return
	}
	if err := validate.RequiredText("name", body.Name, 120); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_name", err.Error())
		return
	}
	if err := validate.OptionalText("phone", body.Phone, 40); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_phone", err.Error())
		return
	}
	owner, err := s.store.UpdateOwnerProfile(r.Context(), claims.Subject, body.Name, body.Phone)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "update_profile_failed", "could not update profile")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, owner)
}

func (s *Server) listPets(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	pets, err := s.store.ListPets(r.Context(), claims.Subject, true)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "pets_error", "could not load pets")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, pets)
}

func (s *Server) getPet(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	pet, err := s.store.GetPet(r.Context(), claims.Subject, chi.URLParam(r, "pet_id"))
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "pet not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, pet)
}

func (s *Server) createPet(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	var input store.PetInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "could not parse request body")
		return
	}
	if err := validatePetInput(input, true); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_pet", err.Error())
		return
	}
	pet, err := s.store.CreatePet(r.Context(), claims.Subject, input)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "create_pet_failed", "could not create pet")
		return
	}
	if claims.Email != "" {
		subject, text, html := email.RegistrationConfirmation(pet.PetName)
		s.sendEmailAsync(r.Context(), email.Message{To: claims.Email, Subject: subject, TextBody: text, HTMLBody: html}, claims.Subject, pet.ChipNormalized)
	}
	httpx.WriteJSON(w, http.StatusCreated, pet)
}

func (s *Server) updatePet(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	var input store.PetInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "could not parse request body")
		return
	}
	if err := validatePetInput(input, false); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_pet", err.Error())
		return
	}
	pet, err := s.store.UpdatePet(r.Context(), claims.Subject, chi.URLParam(r, "pet_id"), input)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "update_pet_failed", "could not update pet")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, pet)
}

func (s *Server) deletePet(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if err := s.store.SoftDeletePet(r.Context(), claims.Subject, chi.URLParam(r, "pet_id")); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "delete_failed", "could not delete pet")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) petLookupHistory(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	history, err := s.store.LookupHistoryForPet(r.Context(), claims.Subject, chi.URLParam(r, "pet_id"))
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "history_failed", "could not load lookup history")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, history)
}

func (s *Server) initiateTransfer(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	var body struct {
		ToEmail string `json:"to_email"`
		Note    string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "could not parse request body")
		return
	}
	if err := validate.Email(strings.ToLower(strings.TrimSpace(body.ToEmail))); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_email", err.Error())
		return
	}
	body.ToEmail = strings.ToLower(strings.TrimSpace(body.ToEmail))
	if err := validate.OptionalText("note", stringPtr(body.Note), 1000); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_note", err.Error())
		return
	}
	pet, err := s.store.GetPet(r.Context(), claims.Subject, chi.URLParam(r, "pet_id"))
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "pet not found")
		return
	}
	token, _, err := s.store.CreateTransfer(r.Context(), pet.ChipPK, claims.Subject, body.ToEmail, "owner", body.Note, time.Now().Add(s.cfg.TransferExpiry))
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "transfer_failed", "could not create transfer")
		return
	}
	link := fmt.Sprintf("%s/transfers/%s/confirm", s.cfg.BaseURL, token)
	subject, text, html := email.TransferInitiated(pet.PetName, body.ToEmail, link)
	s.sendEmailAsync(r.Context(), email.Message{To: claims.Email, Subject: subject, TextBody: text, HTMLBody: html}, claims.Subject, pet.ChipNormalized)
	s.sendEmailAsync(r.Context(), email.Message{
		To:       body.ToEmail,
		Subject:  "OpenChip transfer initiated",
		TextBody: fmt.Sprintf("A transfer for %s has been initiated to this email.", pet.PetName),
		HTMLBody: fmt.Sprintf("<p>A transfer for <strong>%s</strong> has been initiated to this email.</p>", pet.PetName),
	}, "", pet.ChipNormalized)
	httpx.WriteJSON(w, http.StatusCreated, map[string]string{"status": "pending", "token": token})
}

func (s *Server) confirmTransfer(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if err := s.store.ApproveTransfer(r.Context(), token); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "approve_failed", "could not approve transfer")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

func (s *Server) rejectTransfer(w http.ResponseWriter, r *http.Request) {
	if err := s.store.ResolveTransfer(r.Context(), chi.URLParam(r, "token"), "rejected"); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "reject_failed", "could not reject transfer")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}

func (s *Server) exportData(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	data, err := s.store.ExportOwnerData(r.Context(), claims.Subject)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "export_failed", "could not export data")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, data)
}

func (s *Server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if err := s.store.AnonymizeOwner(r.Context(), claims.Subject); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "delete_failed", "could not anonymize account")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"anonymized": true})
}

func (s *Server) createDispute(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ChipID        string `json:"chip_id"`
		ReporterName  string `json:"reporter_name"`
		ReporterEmail string `json:"reporter_email"`
		Description   string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "could not parse request body")
		return
	}
	if err := validate.ChipID(body.ChipID); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_chip_id", err.Error())
		return
	}
	if err := validate.RequiredText("reporter_name", body.ReporterName, 120); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_reporter_name", err.Error())
		return
	}
	if err := validate.Email(strings.ToLower(strings.TrimSpace(body.ReporterEmail))); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_reporter_email", err.Error())
		return
	}
	body.ReporterEmail = strings.ToLower(strings.TrimSpace(body.ReporterEmail))
	if err := validate.RequiredText("description", body.Description, 4000); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_description", err.Error())
		return
	}
	if err := s.store.CreateDispute(r.Context(), body.ChipID, body.ReporterName, body.ReporterEmail, body.Description); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "dispute_failed", "could not create dispute")
		return
	}
	subject, text, html := email.DisputeReceived()
	s.sendEmailAsync(r.Context(), email.Message{To: body.ReporterEmail, Subject: subject, TextBody: text, HTMLBody: html}, "", body.ChipID)
	s.sendEmailAsync(r.Context(), email.Message{
		To:       s.cfg.AdminEmail,
		Subject:  "OpenChip dispute reported",
		TextBody: fmt.Sprintf("A new dispute was submitted for chip %s.", body.ChipID),
		HTMLBody: fmt.Sprintf("<p>A new dispute was submitted for chip <strong>%s</strong>.</p>", body.ChipID),
	}, "", body.ChipID)
	httpx.WriteJSON(w, http.StatusCreated, map[string]bool{"created": true})
}

func (s *Server) aahaLookup(w http.ResponseWriter, r *http.Request) {
	chipID := chi.URLParam(r, "chip_id")
	if err := validate.ChipID(chipID); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_lookup", err.Error())
		return
	}
	results, norm, err := s.store.LookupRegistrations(r.Context(), chipID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_lookup", err.Error())
		return
	}
	registries := []map[string]interface{}{}
	for _, item := range results {
		registries = append(registries, map[string]interface{}{
			"registry":          "OpenChip",
			"manufacturer_hint": item.Manufacturer,
			"contact_email":     s.cfg.SupportEmail,
			"contact_phone":     item.OwnerPhone,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"chip_number":        norm.Normalized,
		"manufacturer_hint":  norm.ManufacturerHint,
		"match_found":        len(results) > 0,
		"participating_data": registries,
	})
}

func (s *Server) adminListDisputes(w http.ResponseWriter, r *http.Request) {
	disputes, err := s.store.ListDisputes(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "disputes_failed", "could not load disputes")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, disputes)
}

func (s *Server) adminGetDispute(w http.ResponseWriter, r *http.Request) {
	dispute, err := s.store.GetDispute(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "dispute not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, dispute)
}

func (s *Server) adminUpdateDispute(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Status         string `json:"status"`
		ResolutionNote string `json:"resolution_note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "could not parse request body")
		return
	}
	switch body.Status {
	case "open", "reviewing", "resolved":
	default:
		httpx.WriteError(w, http.StatusBadRequest, "invalid_status", "status must be open, reviewing, or resolved")
		return
	}
	if err := validate.OptionalText("resolution_note", stringPtr(body.ResolutionNote), 2000); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_resolution_note", err.Error())
		return
	}
	if err := s.store.UpdateDispute(r.Context(), chi.URLParam(r, "id"), body.Status, body.ResolutionNote); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "update_failed", "could not update dispute")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"updated": true})
}

func (s *Server) adminStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.Stats(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "stats_failed", "could not load stats")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, stats)
}

func (s *Server) notifyOwners(ctx context.Context, results []model.LookupRegistration, chipNormalized, agent string) {
	when := time.Now().Format(time.RFC1123)
	seen := map[string]struct{}{}
	for _, item := range results {
		if item.OwnerEmail == nil || *item.OwnerEmail == "" {
			continue
		}
		if _, ok := seen[*item.OwnerEmail]; ok {
			continue
		}
		seen[*item.OwnerEmail] = struct{}{}
		subject, text, html := email.ChipScanned(agent, when)
		s.sendEmailAsync(ctx, email.Message{To: *item.OwnerEmail, Subject: subject, TextBody: text, HTMLBody: html}, item.OwnerID, chipNormalized)
	}
	if err := s.store.MarkLookupNotified(ctx, chipNormalized); err != nil {
		middleware.LoggerFromContext(ctx).Warn("failed to mark lookup notified",
			slog.String("request_id", middleware.RequestIDFromContext(ctx)),
			slog.String("chip_id_normalized", chipNormalized),
			slog.String("error", err.Error()),
		)
	}
}

func (s *Server) allowAuthEmail(email string) bool {
	s.authRateLock.Lock()
	defer s.authRateLock.Unlock()

	now := time.Now()
	window, ok := s.authRates[email]
	if !ok || now.After(window.Expires) {
		s.authRates[email] = rateWindow{Count: 1, Expires: now.Add(time.Minute)}
		return true
	}
	if window.Count >= s.cfg.AuthRatePerMin {
		return false
	}
	window.Count++
	s.authRates[email] = window
	return true
}

func randomHex(bytes int) (string, error) {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func validatePetInput(input store.PetInput, requireChip bool) error {
	if requireChip {
		if err := validate.ChipID(input.ChipID); err != nil {
			return err
		}
	}
	if err := validate.RequiredText("pet_name", input.PetName, 120); err != nil {
		return err
	}
	if err := validate.Species(input.Species); err != nil {
		return err
	}
	if err := validate.OptionalText("breed", input.Breed, 120); err != nil {
		return err
	}
	if err := validate.OptionalText("color", input.Color, 80); err != nil {
		return err
	}
	if err := validate.OptionalText("notes", input.Notes, 2000); err != nil {
		return err
	}
	if err := validate.OptionalText("photo_url", input.PhotoURL, 2048); err != nil {
		return err
	}
	return nil
}

func stringPtr(value string) *string {
	return &value
}

func (s *Server) sendEmailAsync(ctx context.Context, msg email.Message, ownerID, chipID string) {
	requestID := middleware.RequestIDFromContext(ctx)
	logger := middleware.LoggerFromContext(ctx)
	go func() {
		sendCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.email.Send(sendCtx, msg); err != nil {
			logger.Error("email_send_failed",
				slog.String("request_id", requestID),
				slog.String("owner_id", ownerID),
				slog.String("chip_id", chipID),
				slog.String("error", err.Error()),
			)
		}
	}()
}
