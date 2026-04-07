package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"net/smtp"
	"strings"
	"sync"
)

type emailMessage struct {
	To       string
	Subject  string
	HTMLBody string
	TextBody string
}

// EmailSender sends notification emails via SMTP. If the host is empty the
// sender operates as a no-op so the system works without SMTP configured.
type EmailSender struct {
	host     string
	port     int
	username string
	password string
	from     string
	logger   *slog.Logger

	queue chan emailMessage
	wg    sync.WaitGroup
	stop  chan struct{}
}

// NewEmailSender creates an EmailSender. Pass an empty host to disable SMTP.
func NewEmailSender(host string, port int, username, password, from string, logger *slog.Logger) *EmailSender {
	return &EmailSender{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
		logger:   logger,
		queue:    make(chan emailMessage, 256),
		stop:     make(chan struct{}),
	}
}

// Start begins the background goroutine that drains the mail queue.
func (s *EmailSender) Start(_ context.Context) {
	if s.host == "" {
		s.logger.Info("SMTP not configured, email sender disabled")
		return
	}

	s.wg.Add(1)
	go s.run()
	s.logger.Info("email sender started", "host", s.host, "port", s.port)
}

// Stop signals the background sender to drain remaining messages and exit.
func (s *EmailSender) Stop() {
	close(s.stop)
	s.wg.Wait()
}

// Send enqueues an email for background delivery. It returns immediately.
// If the queue is full the message is dropped and an error is logged.
func (s *EmailSender) Send(_ context.Context, to, subject, htmlBody, textBody string) error {
	if s.host == "" {
		return nil
	}

	msg := emailMessage{
		To:       to,
		Subject:  subject,
		HTMLBody: htmlBody,
		TextBody: textBody,
	}

	select {
	case s.queue <- msg:
		return nil
	default:
		s.logger.Error("email queue full, dropping message", "to", to, "subject", subject)
		return fmt.Errorf("email queue full")
	}
}

func (s *EmailSender) run() {
	defer s.wg.Done()
	for {
		select {
		case msg := <-s.queue:
			s.deliver(msg)
		case <-s.stop:
			// Drain remaining messages.
			for {
				select {
				case msg := <-s.queue:
					s.deliver(msg)
				default:
					return
				}
			}
		}
	}
}

// sanitizeHeader removes CR and LF characters to prevent SMTP header injection.
func sanitizeHeader(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	return s
}

func (s *EmailSender) deliver(msg emailMessage) {
	boundary := fmt.Sprintf("VKboundary-%016x", rand.Uint64())

	from := sanitizeHeader(s.from)
	to := sanitizeHeader(msg.To)
	subject := sanitizeHeader(msg.Subject)

	var body strings.Builder
	body.WriteString("From: " + from + "\r\n")
	body.WriteString("To: " + to + "\r\n")
	body.WriteString("Subject: " + subject + "\r\n")
	body.WriteString("MIME-Version: 1.0\r\n")
	body.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n")
	body.WriteString("\r\n")

	// Plain text part.
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	body.WriteString("\r\n")
	body.WriteString(msg.TextBody + "\r\n")

	// HTML part.
	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	body.WriteString("\r\n")
	body.WriteString(msg.HTMLBody + "\r\n")

	body.WriteString("--" + boundary + "--\r\n")

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	var auth smtp.Auth
	if s.username != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}

	if err := smtp.SendMail(addr, auth, s.from, []string{msg.To}, []byte(body.String())); err != nil {
		s.logger.Error("failed to send email", "to", msg.To, "subject", msg.Subject, "error", err)
	} else {
		s.logger.Debug("email sent", "to", msg.To, "subject", msg.Subject)
	}
}
