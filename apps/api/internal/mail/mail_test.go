package mail

import (
	"net/mail"
	"strings"
	"testing"
)

func TestNewSMTPSenderNilWhenUnconfigured(t *testing.T) {
	if NewSMTPSender(SMTPConfig{}) != nil {
		t.Fatal("empty config must yield no sender")
	}
	if NewSMTPSender(SMTPConfig{Host: "smtp.example.com"}) != nil {
		t.Fatal("a host without a From address must yield no sender")
	}
	if NewSMTPSender(SMTPConfig{Host: "smtp.example.com", From: "Prism <noreply@example.com>"}) == nil {
		t.Fatal("host and From must be enough")
	}
}

func TestBuildEncodesHeadersAndParts(t *testing.T) {
	from := &mail.Address{Name: "Prism", Address: "noreply@example.com"}
	to := &mail.Address{Address: "alice@example.com"}
	raw := string(Build(from, to, Message{
		Subject: "Join Acme\r\nBcc: evil@example.com",
		Text:    "Hello\nline two",
		HTML:    "<p>Hello</p>",
	}))

	if strings.Contains(raw, "\nBcc:") {
		t.Fatalf("subject injected a header:\n%s", raw)
	}
	for _, want := range []string{
		"From: \"Prism\" <noreply@example.com>\r\n",
		"To: <alice@example.com>\r\n",
		"multipart/alternative",
		"Content-Type: text/plain; charset=utf-8",
		"Hello\r\nline two",
		"Content-Type: text/html; charset=utf-8",
		"<p>Hello</p>",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("message lacks %q:\n%s", want, raw)
		}
	}

	plain := string(Build(from, to, Message{Subject: "Hi", Text: "x"}))
	if strings.Contains(plain, "multipart") || !strings.Contains(plain, "Content-Type: text/plain") {
		t.Fatalf("text-only message is not single part:\n%s", plain)
	}
}
