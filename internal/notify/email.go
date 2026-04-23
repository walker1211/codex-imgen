package notify

import (
	"context"

	"github.com/walker1211/codex-imgen/internal/config"
	"github.com/walker1211/codex-imgen/internal/store"
)

type FailureMail struct {
	To        string
	JobID     string
	Prompt    string
	LastError string
}

type Dialer interface {
	Send(from string, to string, subject string, body string) error
}

type Mailer struct {
	Config config.EmailConfig
	Dialer Dialer
}

func (m Mailer) SendFailure(msg FailureMail) error {
	if m.Dialer == nil || m.Config.To == "" {
		return nil
	}
	subject := "[codex-imgen] job failed: " + msg.JobID
	body := "job_id: " + msg.JobID + "\nprompt: " + msg.Prompt + "\nerror: " + msg.LastError + "\n"
	return m.Dialer.Send(m.Config.From, msg.To, subject, body)
}

type notificationStore interface {
	GetJob(context.Context, string) (store.Job, []store.JobImage, error)
	UpdateJobNotification(context.Context, string, string) error
}

func NotifyFailureIfNeeded(ctx context.Context, st notificationStore, mailer Mailer, jobID string) error {
	job, images, err := st.GetJob(ctx, jobID)
	if err != nil {
		return err
	}
	if job.Status != "failed" || job.NotificationStatus != "pending" {
		return nil
	}
	lastError := "job failed"
	for _, image := range images {
		if image.LastError != "" {
			lastError = image.LastError
			break
		}
	}
	if err := mailer.SendFailure(FailureMail{To: mailer.Config.To, JobID: job.JobID, Prompt: job.Prompt, LastError: lastError}); err != nil {
		return st.UpdateJobNotification(ctx, jobID, "failed")
	}
	return st.UpdateJobNotification(ctx, jobID, "sent")
}
