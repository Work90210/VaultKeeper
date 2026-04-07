package notifications

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestNewEmailSender_EmptyHost_IsNoop(t *testing.T) {
	sender := NewEmailSender("", 0, "", "", "", noopLogger())
	if sender.host != "" {
		t.Error("expected empty host")
	}

	// Send should return nil immediately when host is empty
	err := sender.Send(context.Background(), "user@example.com", "Subject", "<p>HTML</p>", "Text")
	if err != nil {
		t.Errorf("expected nil error for noop sender, got: %v", err)
	}
}

func TestEmailSender_Send_WithHost_Queues(t *testing.T) {
	sender := NewEmailSender("smtp.example.com", 587, "user", "pass", "from@example.com", noopLogger())

	// Don't start the background worker so messages stay in the queue
	err := sender.Send(context.Background(), "to@example.com", "Subject", "<p>HTML</p>", "Text")
	if err != nil {
		t.Errorf("expected message to be queued, got error: %v", err)
	}

	// Verify message is in the queue
	select {
	case msg := <-sender.queue:
		if msg.To != "to@example.com" {
			t.Errorf("expected To 'to@example.com', got %q", msg.To)
		}
		if msg.Subject != "Subject" {
			t.Errorf("expected Subject 'Subject', got %q", msg.Subject)
		}
		if msg.HTMLBody != "<p>HTML</p>" {
			t.Errorf("expected HTMLBody '<p>HTML</p>', got %q", msg.HTMLBody)
		}
		if msg.TextBody != "Text" {
			t.Errorf("expected TextBody 'Text', got %q", msg.TextBody)
		}
	default:
		t.Error("expected message in queue, but queue was empty")
	}
}

func TestEmailSender_Send_QueueFull(t *testing.T) {
	sender := NewEmailSender("smtp.example.com", 587, "", "", "from@example.com", noopLogger())

	// Fill the queue (capacity 256)
	for i := 0; i < 256; i++ {
		_ = sender.Send(context.Background(), "to@example.com", "Subject", "html", "text")
	}

	// Next send should fail
	err := sender.Send(context.Background(), "to@example.com", "Subject", "html", "text")
	if err == nil {
		t.Error("expected error when queue is full")
	}
}

func TestEmailSender_StartStop_NoHost(t *testing.T) {
	sender := NewEmailSender("", 0, "", "", "", noopLogger())

	// Start with empty host should be a no-op (no goroutine started)
	sender.Start(context.Background())
	// Stop should not hang even though no goroutine was started
	sender.Stop()
}

func TestEmailSender_StartStop_WithHost(t *testing.T) {
	sender := NewEmailSender("smtp.example.com", 587, "", "", "from@example.com", noopLogger())

	sender.Start(context.Background())
	// Stop should drain and return
	sender.Stop()
}

func TestEmailSender_RunProcessesMessages(t *testing.T) {
	sender := NewEmailSender("127.0.0.1", 0, "", "", "from@example.com", noopLogger())

	// Start the sender (background goroutine running)
	sender.Start(context.Background())

	// Send a message while the goroutine is running to exercise the main
	// select case (not the drain path). deliver will fail (no SMTP server)
	// but the run loop processes the message.
	_ = sender.Send(context.Background(), "to@example.com", "Subject", "html", "text")

	// Give the goroutine a moment to process
	time.Sleep(50 * time.Millisecond)

	sender.Stop()
}

func TestEmailSender_StopDrainsQueue(t *testing.T) {
	sender := NewEmailSender("127.0.0.1", 0, "", "", "from@example.com", noopLogger())

	sender.Start(context.Background())

	// Wait for goroutine to be ready and processing
	time.Sleep(20 * time.Millisecond)

	// Fill the queue with multiple messages
	for i := 0; i < 5; i++ {
		_ = sender.Send(context.Background(), "to@example.com", "Subject", "html", "text")
	}

	// Stop should drain remaining messages
	sender.Stop()

	// Queue should be empty after stop
	select {
	case <-sender.queue:
		t.Error("expected queue to be drained after stop")
	default:
		// Queue is empty, as expected
	}
}

func TestEmailSender_DeliverSuccess(t *testing.T) {
	// Start a minimal fake SMTP server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().(*net.TCPAddr)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		fakeSmtp(conn)
	}()

	sender := NewEmailSender("127.0.0.1", addr.Port, "", "", "from@test.com", noopLogger())

	// Deliver directly (bypassing queue)
	sender.deliver(emailMessage{
		To:       "to@test.com",
		Subject:  "Test",
		HTMLBody: "<p>Hello</p>",
		TextBody: "Hello",
	})
}

func TestEmailSender_DeliverWithAuth(t *testing.T) {
	// This tests the auth branch. deliver will fail (no real SMTP with auth)
	// but the PlainAuth line is covered.
	sender := NewEmailSender("127.0.0.1", 0, "user", "pass", "from@test.com", noopLogger())

	sender.deliver(emailMessage{
		To:       "to@test.com",
		Subject:  "Test",
		HTMLBody: "<p>Hello</p>",
		TextBody: "Hello",
	})
}

// fakeSmtp handles one SMTP conversation, accepting all commands.
func fakeSmtp(conn net.Conn) {
	w := bufio.NewWriter(conn)
	r := bufio.NewReader(conn)

	write := func(s string) {
		_, _ = fmt.Fprintf(w, "%s\r\n", s)
		_ = w.Flush()
	}

	write("220 localhost ESMTP")

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			write("250 OK")
		case strings.HasPrefix(cmd, "MAIL FROM"):
			write("250 OK")
		case strings.HasPrefix(cmd, "RCPT TO"):
			write("250 OK")
		case cmd == "DATA":
			write("354 Go ahead")
			// Read until lone "."
			for {
				dataLine, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimSpace(dataLine) == "." {
					break
				}
			}
			write("250 OK")
		case cmd == "QUIT":
			write("221 Bye")
			return
		default:
			write("250 OK")
		}
	}
}

func TestSanitizeHeader(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"clean", "Normal Subject", "Normal Subject"},
		{"with CR", "Subject\rInjection", "SubjectInjection"},
		{"with LF", "Subject\nInjection", "SubjectInjection"},
		{"with CRLF", "Subject\r\nInjection", "SubjectInjection"},
		{"multiple CRLF", "A\r\nB\r\nC", "ABC"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeHeader(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}
