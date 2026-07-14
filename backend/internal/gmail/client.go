package gmail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	gmailapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// Client wraps Gmail API operations used for watch/history monitoring.
type Client struct {
	svc *gmailapi.Service
}

// NewClient builds a Gmail API client from an OAuth2 token source.
func NewClient(ctx context.Context, ts oauth2.TokenSource) (*Client, error) {
	svc, err := gmailapi.NewService(ctx, option.WithTokenSource(ts))
	if err != nil {
		return nil, fmt.Errorf("create gmail service: %w", err)
	}
	return &Client{svc: svc}, nil
}

// Profile returns the authenticated Gmail profile.
func (c *Client) Profile(ctx context.Context) (*gmailapi.Profile, error) {
	return c.svc.Users.GetProfile("me").Context(ctx).Do()
}

// WatchRequest configures users.watch for Pub/Sub push notifications.
type WatchRequest struct {
	TopicName string
	LabelIDs  []string
}

// WatchResult is returned by users.watch.
type WatchResult struct {
	HistoryID  string
	Expiration time.Time
}

// StartWatch registers a Gmail push notification watch on the mailbox.
func (c *Client) StartWatch(ctx context.Context, req WatchRequest) (*WatchResult, error) {
	body := &gmailapi.WatchRequest{
		TopicName: req.TopicName,
		LabelIds:  req.LabelIDs,
	}
	resp, err := c.svc.Users.Watch("me", body).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("gmail watch: %w", err)
	}
	return &WatchResult{
		HistoryID:  strconv.FormatUint(resp.HistoryId, 10),
		Expiration: time.UnixMilli(resp.Expiration).UTC(),
	}, nil
}

// StopWatch stops mailbox push notifications.
func (c *Client) StopWatch(ctx context.Context) error {
	return c.svc.Users.Stop("me").Context(ctx).Do()
}

// MessageSummary is a lightweight view of a Gmail message for notifications.
type MessageSummary struct {
	ID          string
	ThreadID    string
	From        string
	Subject     string
	Snippet     string
	InternalDate time.Time
	LabelIDs    []string
}

// ListHistorySince returns new message IDs added since startHistoryID.
func (c *Client) ListHistorySince(ctx context.Context, startHistoryID uint64) (newHistoryID uint64, messageIDs []string, err error) {
	newHistoryID = startHistoryID
	pageToken := ""
	seen := map[string]struct{}{}

	for {
		call := c.svc.Users.History.List("me").
			StartHistoryId(startHistoryID).
			HistoryTypes("messageAdded").
			Context(ctx)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		resp, err := call.Do()
		if err != nil {
			return 0, nil, fmt.Errorf("gmail history list: %w", err)
		}
		if resp.HistoryId != 0 {
			newHistoryID = resp.HistoryId
		}
		for _, h := range resp.History {
			for _, m := range h.MessagesAdded {
				if m.Message == nil || m.Message.Id == "" {
					continue
				}
				if _, ok := seen[m.Message.Id]; ok {
					continue
				}
				seen[m.Message.Id] = struct{}{}
				messageIDs = append(messageIDs, m.Message.Id)
			}
		}
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return newHistoryID, messageIDs, nil
}

// GetMessage fetches metadata for a message suitable for push notifications.
func (c *Client) GetMessage(ctx context.Context, messageID string) (*MessageSummary, error) {
	msg, err := c.svc.Users.Messages.Get("me", messageID).
		Format("metadata").
		MetadataHeaders("From", "Subject", "Date").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("gmail get message: %w", err)
	}
	summary := &MessageSummary{
		ID:       msg.Id,
		ThreadID: msg.ThreadId,
		Snippet:  msg.Snippet,
		LabelIDs: msg.LabelIds,
	}
	if msg.InternalDate > 0 {
		summary.InternalDate = time.UnixMilli(msg.InternalDate).UTC()
	}
	for _, h := range msg.Payload.Headers {
		switch strings.ToLower(h.Name) {
		case "from":
			summary.From = h.Value
		case "subject":
			summary.Subject = h.Value
		}
	}
	return summary, nil
}

// PubSubNotification is the payload Google Pub/Sub delivers for Gmail push.
type PubSubNotification struct {
	EmailAddress string `json:"emailAddress"`
	HistoryID    uint64 `json:"historyId"`
}

// DecodePubSubData decodes the base64 Pub/Sub message data field.
func DecodePubSubData(data string) (*PubSubNotification, error) {
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		// Pub/Sub sometimes sends URL-safe base64
		raw, err = base64.URLEncoding.DecodeString(data)
		if err != nil {
			return nil, fmt.Errorf("decode pubsub data: %w", err)
		}
	}
	var n PubSubNotification
	if err := json.Unmarshal(raw, &n); err != nil {
		return nil, fmt.Errorf("unmarshal pubsub data: %w", err)
	}
	return &n, nil
}

// ParseHistoryID converts a stored string history ID to uint64.
func ParseHistoryID(s string) (uint64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty history id")
	}
	return strconv.ParseUint(s, 10, 64)
}
