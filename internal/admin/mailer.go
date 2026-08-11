package admin

import (
	"fmt"
	"log"
	"os"

	smtp "project-tap/internal/pkg/smtpbody"

	"gopkg.in/gomail.v2"
)

// SendMerchantApprovedEmail sends an email when a merchant is approved.
func SendMerchantApprovedEmail(email, name string) error {
	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "http://localhost:3000"
	}
	return sendMail(email, "Unicard Application Approved", fmt.Sprintf(smtp.MerchantApprovedEmail(), name, appURL))
}

// SendMerchantRejectedEmail sends an email when a merchant is rejected.
func SendMerchantRejectedEmail(email, name, reason string) error {
	return sendMail(email, "Unicard Application Update", fmt.Sprintf(smtp.MerchantRejectedEmail(), name, reason))
}

// SendMerchantSuspendedEmail sends an email when a merchant is suspended.
func SendMerchantSuspendedEmail(email, name, reason string) error {
	return sendMail(email, "Unicard Account Suspended", fmt.Sprintf(smtp.MerchantSuspendedEmail(), name, reason))
}

// SendMerchantDeletedEmail sends an email when a merchant is deleted.
func SendMerchantDeletedEmail(email, name string) error {
	return sendMail(email, "Unicard Account Deleted", fmt.Sprintf(smtp.MerchantDeletedEmail(), name))
}

// internal helper

// sendMail configures and dispatches an HTML email using the system SMTP settings.
func sendMail(to, subject, htmlBody string) error {
	host := os.Getenv("SMTP_HOST")
	email := os.Getenv("SMTP_EMAIL")
	sender := os.Getenv("SMTP_SENDER")
	pass := os.Getenv("SMTP_PASSWORD")

	m := gomail.NewMessage()
	m.SetHeader("From", sender+" <"+email+">")
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)
	m.SetBody("text/html", htmlBody)

	d := gomail.NewDialer(host, 587, email, pass)
	err := d.DialAndSend(m)
	if err != nil {
		log.Printf("sendMail: failed for %s: %v", to, err)
	}
	return err
}