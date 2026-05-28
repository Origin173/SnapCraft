package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"time"

	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/snapshot"
)

// Notifier sends backup result notifications.
type Notifier interface {
	NotifySuccess(m *snapshot.Manifest)
	NotifyFailure(m *snapshot.Manifest, err error)
	NotifyRestore(m *snapshot.Manifest)
}

// MultiNotifier chains multiple notifiers.
type MultiNotifier struct {
	notifiers []Notifier
}

func NewMulti(notifiers ...Notifier) *MultiNotifier {
	return &MultiNotifier{notifiers: notifiers}
}

func (m *MultiNotifier) NotifySuccess(manifest *snapshot.Manifest) {
	for _, n := range m.notifiers {
		n.NotifySuccess(manifest)
	}
}

func (m *MultiNotifier) NotifyFailure(manifest *snapshot.Manifest, err error) {
	for _, n := range m.notifiers {
		n.NotifyFailure(manifest, err)
	}
}

func (m *MultiNotifier) NotifyRestore(manifest *snapshot.Manifest) {
	for _, n := range m.notifiers {
		n.NotifyRestore(manifest)
	}
}

// NoopNotifier discards notifications.
type NoopNotifier struct{}

func (NoopNotifier) NotifySuccess(m *snapshot.Manifest)              {}
func (NoopNotifier) NotifyFailure(m *snapshot.Manifest, err error)   {}
func (NoopNotifier) NotifyRestore(m *snapshot.Manifest)              {}

// WebhookNotifier posts JSON payloads to a webhook URL.
type WebhookNotifier struct {
	cfg    *config.Config
	client *http.Client
}

func NewWebhookNotifier(cfg *config.Config) *WebhookNotifier {
	return &WebhookNotifier{
		cfg:    cfg,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

type webhookPayload struct {
	Event      string    `json:"event"`
	Server     string    `json:"server"`
	SnapshotID string    `json:"snapshot_id"`
	Status     string    `json:"status"`
	Mode       string    `json:"mode"`
	TotalBytes int64     `json:"total_bytes"`
	StartedAt  time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	Error      string    `json:"error,omitempty"`
}

func (w *WebhookNotifier) NotifySuccess(m *snapshot.Manifest) {
	w.send("backup.success", m, "")
}

func (w *WebhookNotifier) NotifyFailure(m *snapshot.Manifest, err error) {
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	w.send("backup.failure", m, msg)
}

func (w *WebhookNotifier) NotifyRestore(m *snapshot.Manifest) {
	w.send("restore.success", m, "")
}

func (w *WebhookNotifier) send(event string, m *snapshot.Manifest, errMsg string) {
	if !w.cfg.Notify.Webhook.Enabled || w.cfg.Notify.Webhook.URL == "" {
		return
	}
	payload := webhookPayload{
		Event:      event,
		Server:     m.ServerName,
		SnapshotID: m.ID,
		Status:     m.Status,
		Mode:       m.Mode,
		TotalBytes: m.TotalBytes,
		StartedAt:  m.StartedAt,
		CompletedAt: m.CompletedAt,
		Error:      errMsg,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, w.cfg.Notify.Webhook.URL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	w.client.Do(req)
}

// EmailNotifier sends email via SMTP.
type EmailNotifier struct {
	cfg *config.Config
}

func NewEmailNotifier(cfg *config.Config) *EmailNotifier {
	return &EmailNotifier{cfg: cfg}
}

func (e *EmailNotifier) NotifySuccess(m *snapshot.Manifest) {
	e.send(fmt.Sprintf("[SnapCraft] Backup success: %s", m.ID), fmt.Sprintf("Server %s backup %s completed.", m.ServerName, m.ID))
}

func (e *EmailNotifier) NotifyFailure(m *snapshot.Manifest, err error) {
	e.send(fmt.Sprintf("[SnapCraft] Backup failed: %s", m.ID), fmt.Sprintf("Server %s backup failed: %v", m.ServerName, err))
}

func (e *EmailNotifier) NotifyRestore(m *snapshot.Manifest) {
	e.send(fmt.Sprintf("[SnapCraft] Restore success: %s", m.ID), fmt.Sprintf("Server %s restored from %s.", m.ServerName, m.ID))
}

func (e *EmailNotifier) send(subject, body string) {
	if !e.cfg.Notify.Email.Enabled {
		return
	}
	addr := fmt.Sprintf("%s:%d", e.cfg.Notify.Email.SMTPHost, e.cfg.Notify.Email.SMTPPort)
	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", e.cfg.Notify.Email.To, subject, body))
	auth := smtp.PlainAuth("", e.cfg.Notify.Email.Username, e.cfg.Notify.Email.Password, e.cfg.Notify.Email.SMTPHost)
	_ = smtp.SendMail(addr, auth, e.cfg.Notify.Email.From, []string{e.cfg.Notify.Email.To}, msg)
}

// BuildFromConfig creates notifiers from configuration.
func BuildFromConfig(cfg *config.Config) Notifier {
	var notifiers []Notifier
	if cfg.Notify.Webhook.Enabled {
		notifiers = append(notifiers, NewWebhookNotifier(cfg))
	}
	if cfg.Notify.Email.Enabled {
		notifiers = append(notifiers, NewEmailNotifier(cfg))
	}
	if len(notifiers) == 0 {
		return NoopNotifier{}
	}
	return NewMulti(notifiers...)
}
