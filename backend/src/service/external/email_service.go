package external

import (
	"context"
	"fmt"

	"github.com/resend/resend-go/v2"
)

type EmailService struct {
	client    *resend.Client
	fromEmail string
	fromName  string
}

func NewEmailService(apiKey, fromEmail, fromName string) *EmailService {
	return &EmailService{
		client:    resend.NewClient(apiKey),
		fromEmail: fromEmail,
		fromName:  fromName,
	}
}

type SendEmailInput struct {
	To      string
	Subject string
	HTML    string
}

func (s *EmailService) Send(ctx context.Context, input SendEmailInput) error {
	_, err := s.client.Emails.SendWithContext(ctx, &resend.SendEmailRequest{
		From:    fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail),
		To:      []string{input.To},
		Subject: input.Subject,
		Html:    input.HTML,
	})
	return err
}
