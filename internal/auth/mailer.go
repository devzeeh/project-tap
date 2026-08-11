package auth

import (
	"fmt"
	"log"
	"os"

	smtp "project-tap/internal/pkg/smtpbody"

	"gopkg.in/gomail.v2"
)

// SendOTPEmail sends a password-reset OTP to the given address.
func SendOTPEmail(email, name, otp string) error {
	return sendMail(email, "Unicard Password Reset OTP", fmt.Sprintf(smtp.OTPCode(), name, otp))
}

// SendPasswordChangedEmail notifies the user that their password was changed.
func SendPasswordChangedEmail(email, name string) error {
	return sendMail(email, "Unicard Password Changed", fmt.Sprintf(smtp.PasswordChangedEmail(), name))
}

// SendSignupOTPEmail sends an email-verification OTP during signup.
func SendSignupOTPEmail(email, name, otp string) error {
	return sendMail(email, "Verify Your Email Address", fmt.Sprintf(smtp.SignupOTPCode(), name, otp))
}

// SendWelcomeEmail sends the post-registration welcome message.
func SendWelcomeEmail(email, name string) error {
	appURL := os.Getenv("APP_URL")
	if appURL == "" {
		appURL = "http://localhost:3000"
	}
	err := sendMail(email, "Welcome to Unicard!", fmt.Sprintf(smtp.WelcomeEmail(), name, appURL))
	if err != nil {
		log.Printf("SendWelcomeEmail: failed for %s: %v", email, err)
	}
	return err
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
	return d.DialAndSend(m)
}