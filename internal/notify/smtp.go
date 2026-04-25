package notify

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/walker1211/codex-imgen/internal/config"
)

type SMTPDialer struct {
	Config   config.EmailConfig
	SendFunc func(addr string, auth smtp.Auth, from string, to []string, msg []byte) error
	Sleep    func(time.Duration)
}

func NewMailer(cfg config.EmailConfig) Mailer {
	mailer := Mailer{Config: cfg}
	if cfg.Enabled {
		mailer.Dialer = SMTPDialer{Config: cfg}
	}
	return mailer
}

func ValidateEmailConfig(cfg config.EmailConfig) error {
	if !cfg.Enabled {
		return nil
	}
	if strings.TrimSpace(cfg.SMTPHost) == "" {
		return errors.New("email.smtp_host is required")
	}
	if cfg.SMTPPort <= 0 || cfg.SMTPPort > 65535 {
		return errors.New("email.smtp_port must be between 1 and 65535")
	}
	if strings.TrimSpace(cfg.From) == "" {
		return errors.New("email.from is required")
	}
	if strings.TrimSpace(cfg.To) == "" {
		return errors.New("email.to is required")
	}
	if strings.TrimSpace(cfg.SMTPAuthCode) == "" {
		return errors.New("EMAIL_SMTP_AUTH_CODE is required when email is enabled")
	}
	if cfg.Timeout <= 0 {
		return errors.New("email.timeout must be positive")
	}
	if cfg.RetryTimes < 1 {
		return errors.New("email.retry_times must be at least 1")
	}
	if cfg.RetryWaitTime < 0 {
		return errors.New("email.retry_wait_time must not be negative")
	}
	if cfg.UseProxy {
		return errors.New("email.use_proxy is not supported yet")
	}
	return nil
}

func (d SMTPDialer) Send(from string, to string, subject string, body string) error {
	if err := ValidateEmailConfig(d.Config); err != nil {
		return err
	}
	msg := buildMessage(from, to, subject, body)
	attempts := d.Config.RetryTimes
	if attempts <= 0 {
		attempts = 1
	}
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := d.sendOnce(from, to, msg); err != nil {
			lastErr = err
			if attempt < attempts {
				sleep := d.Sleep
				if sleep == nil {
					sleep = time.Sleep
				}
				sleep(d.Config.RetryWaitTime)
				continue
			}
			break
		}
		return nil
	}
	return fmt.Errorf("send email after %d attempts: %w", attempts, lastErr)
}

func (d SMTPDialer) sendOnce(from string, to string, msg []byte) error {
	addr := fmt.Sprintf("%s:%d", d.Config.SMTPHost, d.Config.SMTPPort)
	auth := smtp.PlainAuth("", d.Config.From, d.Config.SMTPAuthCode, d.Config.SMTPHost)
	if d.SendFunc != nil {
		return d.SendFunc(addr, auth, from, []string{to}, msg)
	}
	if d.Config.SMTPPort == 465 {
		return sendMailTLS(addr, auth, from, []string{to}, msg, d.Config)
	}
	return sendMail(addr, auth, from, []string{to}, msg, d.Config)
}

func buildMessage(from string, to string, subject string, body string) []byte {
	var builder strings.Builder
	builder.WriteString("From: ")
	builder.WriteString(from)
	builder.WriteString("\r\nTo: ")
	builder.WriteString(to)
	builder.WriteString("\r\nSubject: ")
	builder.WriteString(subject)
	builder.WriteString("\r\nMIME-Version: 1.0")
	builder.WriteString("\r\nContent-Type: text/plain; charset=utf-8")
	builder.WriteString("\r\n\r\n")
	builder.WriteString(body)
	return []byte(builder.String())
}

func sendMail(addr string, auth smtp.Auth, from string, to []string, msg []byte, cfg config.EmailConfig) error {
	dialer := net.Dialer{Timeout: cfg.Timeout}
	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(cfg.Timeout)); err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, cfg.SMTPHost)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.Auth(auth); err != nil {
		return err
	}
	return sendSMTPData(client, from, to, msg)
}

func sendMailTLS(addr string, auth smtp.Auth, from string, to []string, msg []byte, cfg config.EmailConfig) error {
	dialer := net.Dialer{Timeout: cfg.Timeout}
	conn, err := tls.DialWithDialer(&dialer, "tcp", addr, &tls.Config{ServerName: cfg.SMTPHost})
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(cfg.Timeout)); err != nil {
		return err
	}
	client, err := smtp.NewClient(conn, cfg.SMTPHost)
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.Auth(auth); err != nil {
		return err
	}
	if err := sendSMTPData(client, from, to, msg); err != nil {
		return err
	}
	return client.Quit()
}

func sendSMTPData(client *smtp.Client, from string, to []string, msg []byte) error {
	if err := client.Mail(from); err != nil {
		return err
	}
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return err
		}
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(msg); err != nil {
		_ = writer.Close()
		return err
	}
	return writer.Close()
}
