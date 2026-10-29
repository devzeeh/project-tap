package user

import (
	"fmt"
	smtp "project-tap/internal/pkg/smtpbody"

	"gopkg.in/gomail.v2"
)

type SMTPMailer struct {
	Host   string
	Port   int
	Email  string
	Sender string
	Pass   string
}

func NewSMTPMailerFromEnv(host, sender, email, pass string, port int) *SMTPMailer {
	return &SMTPMailer{Host: host, Port: port, Email: email, Sender: sender, Pass: pass}
}

func (m *SMTPMailer) SendEmailChangeVerification(to, newEmail, name, verifyURL string) error {
	if m.Host == "" || m.Email == "" {
		return fmt.Errorf("SMTP credentials not configured")
	}

	msg := gomail.NewMessage()
	msg.SetHeader("From", m.Sender+" <"+m.Email+">")
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", "Approve Your Email Change")
	msg.SetBody("text/html", fmt.Sprintf(smtp.EmailVerificationBody(), name, newEmail, verifyURL))

	d := gomail.NewDialer(m.Host, m.Port, m.Email, m.Pass)
	return d.DialAndSend(msg)
}