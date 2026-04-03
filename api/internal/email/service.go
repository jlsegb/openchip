package email

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/resend/resend-go/v2"
)

type Service struct {
	client *resend.Client
	from   string
	logger *slog.Logger
	disabled bool
}

type Message struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

func New(apiKey, from string, disabled bool, logger *slog.Logger) *Service {
	if disabled || apiKey == "" {
		return &Service{from: from, logger: logger, disabled: true}
	}
	return &Service{client: resend.NewClient(apiKey), from: from, logger: logger}
}

func (s *Service) Send(ctx context.Context, msg Message) error {
	if s.disabled || s.client == nil {
		s.logger.Info("email_stubbed", slog.String("to_hint", redactEmail(msg.To)), slog.String("subject", msg.Subject))
		return nil
	}

	_, err := s.client.Emails.SendWithContext(ctx, &resend.SendEmailRequest{
		From:    s.from,
		To:      []string{msg.To},
		Subject: msg.Subject,
		Text:    msg.TextBody,
		Html:    msg.HTMLBody,
	})
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	return nil
}

func redactEmail(address string) string {
	parts := strings.Split(address, "@")
	if len(parts) != 2 || parts[0] == "" {
		return "redacted"
	}
	return parts[0][:1] + "***@" + parts[1]
}

func MagicLink(link string) (string, string, string) {
	subject := "Click to sign in to OpenChip"
	text := "Use this one-time sign-in link for OpenChip:\n\n" + link + "\n\nThis link expires in 15 minutes."
	html := "<p>Use this one-time sign-in link for OpenChip:</p><p><a href=\"" + link + "\">Sign in</a></p><p>This link expires in 15 minutes.</p>"
	return subject, text, html
}

func RegistrationConfirmation(name string) (string, string, string) {
	subject := fmt.Sprintf("Your pet %s is now registered", name)
	text := fmt.Sprintf("%s is now registered with OpenChip.", name)
	html := fmt.Sprintf("<p><strong>%s</strong> is now registered with OpenChip.</p>", name)
	return subject, text, html
}

func ChipScanned(agent, when string) (string, string, string) {
	subject := "Your pet's chip was just scanned"
	text := fmt.Sprintf("Your pet's chip was scanned at %s on %s. If this was unexpected, contact us immediately.", agent, when)
	html := fmt.Sprintf("<p>Your pet's chip was scanned at <strong>%s</strong> on %s. If this was unexpected, contact us immediately.</p>", agent, when)
	return subject, text, html
}

func TransferInitiated(name, email, link string) (string, string, string) {
	subject := fmt.Sprintf("Confirm transfer of %s to %s", name, email)
	text := fmt.Sprintf("Confirm the transfer of %s to %s:\n\n%s", name, email, link)
	html := fmt.Sprintf("<p>Confirm the transfer of <strong>%s</strong> to %s.</p><p><a href=\"%s\">Confirm transfer</a></p>", name, email, link)
	return subject, text, html
}

func TransferApproved(name string) (string, string, string) {
	subject := "Transfer approved"
	text := fmt.Sprintf("The transfer for %s has been approved.", name)
	html := fmt.Sprintf("<p>The transfer for <strong>%s</strong> has been approved.</p>", name)
	return subject, text, html
}

func DisputeReceived() (string, string, string) {
	subject := "Dispute received"
	text := "We received your dispute report and will review it."
	html := "<p>We received your dispute report and will review it.</p>"
	return subject, text, html
}
