package fcm

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// Notifier sends push notifications via Firebase Cloud Messaging.
type Notifier struct {
	client *messaging.Client
	log    *slog.Logger
	noop   bool
}

// New creates an FCM notifier. If credentialsFile is missing, a no-op notifier is returned (useful in local/dev).
func New(ctx context.Context, credentialsFile string, log *slog.Logger) (*Notifier, error) {
	if credentialsFile == "" {
		log.Warn("FCM disabled: no credentials file configured")
		return &Notifier{log: log, noop: true}, nil
	}
	if _, err := os.Stat(credentialsFile); err != nil {
		log.Warn("FCM disabled: credentials file not found", "path", credentialsFile)
		return &Notifier{log: log, noop: true}, nil
	}

	app, err := firebase.NewApp(ctx, nil, option.WithCredentialsFile(credentialsFile))
	if err != nil {
		return nil, fmt.Errorf("firebase app: %w", err)
	}
	client, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("fcm messaging: %w", err)
	}
	return &Notifier{client: client, log: log}, nil
}

// PushPayload is the data delivered to the mobile app.
type PushPayload struct {
	Title    string
	Body     string
	Data     map[string]string
	Token    string
}

// Send delivers a notification to a single FCM registration token.
func (n *Notifier) Send(ctx context.Context, p PushPayload) error {
	if n.noop {
		n.log.Info("FCM noop send", "title", p.Title, "token_prefix", truncate(p.Token, 12))
		return nil
	}
	msg := &messaging.Message{
		Token: p.Token,
		Notification: &messaging.Notification{
			Title: p.Title,
			Body:  p.Body,
		},
		Data: p.Data,
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				ChannelID: "gmail_notifications",
				Sound:     "default",
			},
		},
	}
	id, err := n.client.Send(ctx, msg)
	if err != nil {
		return fmt.Errorf("fcm send: %w", err)
	}
	n.log.Info("FCM sent", "message_id", id)
	return nil
}

// SendMulticast sends to multiple tokens and returns invalid tokens to prune.
func (n *Notifier) SendMulticast(ctx context.Context, tokens []string, title, body string, data map[string]string) (invalid []string, err error) {
	if len(tokens) == 0 {
		return nil, nil
	}
	if n.noop {
		n.log.Info("FCM noop multicast", "count", len(tokens), "title", title)
		return nil, nil
	}
	msg := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
		Android: &messaging.AndroidConfig{
			Priority: "high",
			Notification: &messaging.AndroidNotification{
				ChannelID: "gmail_notifications",
				Sound:     "default",
			},
		},
	}
	resp, err := n.client.SendEachForMulticast(ctx, msg)
	if err != nil {
		return nil, err
	}
	for i, r := range resp.Responses {
		if r.Success {
			continue
		}
		n.log.Warn("FCM send failed", "error", r.Error, "index", i)
		if messaging.IsUnregistered(r.Error) || messaging.IsInvalidArgument(r.Error) {
			invalid = append(invalid, tokens[i])
		}
	}
	return invalid, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
