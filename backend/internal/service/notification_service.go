package service

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yashs/mobile-gmail-notification/internal/domain"
	"github.com/yashs/mobile-gmail-notification/internal/fcm"
	gmailclient "github.com/yashs/mobile-gmail-notification/internal/gmail"
	"github.com/yashs/mobile-gmail-notification/pkg/apperrors"
)

// NotificationService stores history and drives FCM delivery.
type NotificationService struct {
	notifications domain.NotificationRepository
	settings      domain.SettingsRepository
	devices       domain.DeviceTokenRepository
	accounts      *GmailAccountService
	fcm           *fcm.Notifier
	log           *slog.Logger
}

func NewNotificationService(
	notifications domain.NotificationRepository,
	settings domain.SettingsRepository,
	devices domain.DeviceTokenRepository,
	accounts *GmailAccountService,
	notifier *fcm.Notifier,
	log *slog.Logger,
) *NotificationService {
	return &NotificationService{
		notifications: notifications,
		settings:      settings,
		devices:       devices,
		accounts:      accounts,
		fcm:           notifier,
		log:           log,
	}
}

func (s *NotificationService) List(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*domain.NotificationRecord, error) {
	return s.notifications.ListByUser(ctx, userID, limit, offset)
}

func (s *NotificationService) MarkRead(ctx context.Context, userID, id uuid.UUID) error {
	return s.notifications.MarkRead(ctx, id, userID)
}

func (s *NotificationService) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return s.notifications.MarkAllRead(ctx, userID)
}

func (s *NotificationService) UnreadCount(ctx context.Context, userID uuid.UUID) (int, error) {
	return s.notifications.CountUnread(ctx, userID)
}

func (s *NotificationService) GetSettings(ctx context.Context, userID uuid.UUID) (*domain.NotificationSettings, error) {
	st, err := s.settings.GetByUser(ctx, userID)
	if err != nil {
		if err == apperrors.ErrNotFound {
			st = &domain.NotificationSettings{
				UserID:          userID,
				Enabled:         true,
				QuietHoursStart: "22:00",
				QuietHoursEnd:   "07:00",
				OnlyPrimary:     true,
			}
			_ = s.settings.Upsert(ctx, st)
			return st, nil
		}
		return nil, err
	}
	return st, nil
}

func (s *NotificationService) UpdateSettings(ctx context.Context, userID uuid.UUID, in *domain.NotificationSettings) (*domain.NotificationSettings, error) {
	in.UserID = userID
	if err := s.settings.Upsert(ctx, in); err != nil {
		return nil, apperrors.Wrap(err, "internal_error", "failed to save settings", 500)
	}
	return s.GetSettings(ctx, userID)
}

func (s *NotificationService) RegisterDevice(ctx context.Context, userID uuid.UUID, token, platform string) error {
	if token == "" {
		return apperrors.Wrap(nil, "validation_error", "fcm token required", 400)
	}
	if platform == "" {
		platform = "android"
	}
	return s.devices.Upsert(ctx, &domain.DeviceToken{
		UserID:   userID,
		Token:    token,
		Platform: platform,
	})
}

func (s *NotificationService) UnregisterDevice(ctx context.Context, userID uuid.UUID, token string) error {
	return s.devices.Delete(ctx, userID, token)
}

// SyncAccount processes Gmail history since the stored historyId and pushes FCM for new messages.
func (s *NotificationService) SyncAccount(ctx context.Context, account *domain.GmailAccount) error {
	if !account.IsActive || !account.NotificationsOn {
		return nil
	}

	settings, err := s.GetSettings(ctx, account.UserID)
	if err != nil {
		return err
	}
	if !settings.Enabled {
		return nil
	}

	startID, err := gmailclient.ParseHistoryID(account.HistoryID)
	if err != nil || startID == 0 {
		s.log.Warn("invalid history id; skipping sync", "account", account.Email, "history_id", account.HistoryID)
		return nil
	}

	client, err := s.accounts.gmailClientFor(ctx, account)
	if err != nil {
		return err
	}

	newHID, messageIDs, err := client.ListHistorySince(ctx, startID)
	if err != nil {
		return err
	}

	for _, mid := range messageIDs {
		msg, err := client.GetMessage(ctx, mid)
		if err != nil {
			s.log.Warn("failed to fetch message", "message_id", mid, "error", err)
			continue
		}
		if !s.shouldNotify(settings, msg) {
			continue
		}
		if inQuietHours(settings, time.Now().UTC()) {
			// Still store history, but skip push during quiet hours.
			_ = s.storeNotification(ctx, account, msg)
			continue
		}
		if err := s.storeAndPush(ctx, account, msg); err != nil {
			s.log.Warn("failed to notify", "message_id", mid, "error", err)
		}
	}

	account.HistoryID = strconv.FormatUint(newHID, 10)
	now := time.Now().UTC()
	account.LastSyncedAt = &now
	return s.accounts.UpdateAccount(ctx, account)
}

// HandlePubSub processes a Gmail Pub/Sub push callback.
func (s *NotificationService) HandlePubSub(ctx context.Context, emailAddress string, historyID uint64) error {
	account, err := s.accounts.FindByEmail(ctx, emailAddress)
	if err != nil {
		s.log.Warn("pubsub for unknown account", "email", emailAddress)
		return nil
	}
	// Advance only after sync; historyID from push is the new watermark hint.
	_ = historyID
	return s.SyncAccount(ctx, account)
}

func (s *NotificationService) storeAndPush(ctx context.Context, account *domain.GmailAccount, msg *gmailclient.MessageSummary) error {
	if err := s.storeNotification(ctx, account, msg); err != nil {
		return err
	}
	devices, err := s.devices.ListByUser(ctx, account.UserID)
	if err != nil {
		return err
	}
	tokens := make([]string, 0, len(devices))
	for _, d := range devices {
		tokens = append(tokens, d.Token)
	}
	title := msg.From
	if title == "" {
		title = account.Email
	}
	body := msg.Subject
	if body == "" {
		body = msg.Snippet
	}
	data := map[string]string{
		"message_id":       msg.ID,
		"thread_id":        msg.ThreadID,
		"gmail_account_id": account.ID.String(),
		"from":             msg.From,
		"subject":          msg.Subject,
	}
	invalid, err := s.fcm.SendMulticast(ctx, tokens, title, body, data)
	if err != nil {
		return err
	}
	for _, t := range invalid {
		_ = s.devices.DeleteByToken(ctx, t)
	}
	return nil
}

func (s *NotificationService) storeNotification(ctx context.Context, account *domain.GmailAccount, msg *gmailclient.MessageSummary) error {
	rec := &domain.NotificationRecord{
		UserID:         account.UserID,
		GmailAccountID: account.ID,
		MessageID:      msg.ID,
		ThreadID:       msg.ThreadID,
		FromAddress:    msg.From,
		Subject:        msg.Subject,
		Snippet:        msg.Snippet,
		ReceivedAt:     msg.InternalDate,
	}
	if rec.ReceivedAt.IsZero() {
		rec.ReceivedAt = time.Now().UTC()
	}
	return s.notifications.Create(ctx, rec)
}

func (s *NotificationService) shouldNotify(settings *domain.NotificationSettings, msg *gmailclient.MessageSummary) bool {
	labels := map[string]struct{}{}
	for _, l := range msg.LabelIDs {
		labels[l] = struct{}{}
	}
	if _, spam := labels["SPAM"]; spam && !settings.IncludeSpam {
		return false
	}
	if settings.OnlyPrimary {
		if _, inbox := labels["INBOX"]; !inbox {
			return false
		}
		// CATEGORY_PERSONAL approximates primary when categories exist.
		hasCategory := false
		for l := range labels {
			if strings.HasPrefix(l, "CATEGORY_") {
				hasCategory = true
				break
			}
		}
		if hasCategory {
			if _, primary := labels["CATEGORY_PERSONAL"]; !primary {
				return false
			}
		}
	}
	if kw := strings.TrimSpace(settings.KeywordFilter); kw != "" {
		hay := strings.ToLower(msg.Subject + " " + msg.Snippet + " " + msg.From)
		if !strings.Contains(hay, strings.ToLower(kw)) {
			return false
		}
	}
	if allow := strings.TrimSpace(settings.SenderAllowlist); allow != "" {
		from := strings.ToLower(msg.From)
		matched := false
		for _, part := range strings.Split(allow, ",") {
			part = strings.TrimSpace(strings.ToLower(part))
			if part != "" && strings.Contains(from, part) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}

func inQuietHours(settings *domain.NotificationSettings, now time.Time) bool {
	if !settings.QuietHoursEnabled {
		return false
	}
	start, err1 := parseHHMM(settings.QuietHoursStart)
	end, err2 := parseHHMM(settings.QuietHoursEnd)
	if err1 != nil || err2 != nil {
		return false
	}
	mins := now.Hour()*60 + now.Minute()
	if start <= end {
		return mins >= start && mins < end
	}
	// Overnight window e.g. 22:00–07:00
	return mins >= start || mins < end
}

func parseHHMM(s string) (int, error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, apperrors.ErrValidation
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	return h*60 + m, nil
}
