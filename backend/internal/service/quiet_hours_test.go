package service

import (
	"testing"
	"time"

	"github.com/yashs/mobile-gmail-notification/internal/domain"
	gmailclient "github.com/yashs/mobile-gmail-notification/internal/gmail"
)

func TestInQuietHoursOvernight(t *testing.T) {
	s := &domain.NotificationSettings{
		QuietHoursEnabled: true,
		QuietHoursStart:   "22:00",
		QuietHoursEnd:     "07:00",
	}
	// 23:30 UTC — inside
	at := time.Date(2026, 7, 14, 23, 30, 0, 0, time.UTC)
	if !inQuietHours(s, at) {
		t.Fatal("expected quiet at 23:30")
	}
	// 08:00 UTC — outside
	at = time.Date(2026, 7, 14, 8, 0, 0, 0, time.UTC)
	if inQuietHours(s, at) {
		t.Fatal("expected not quiet at 08:00")
	}
}

func TestShouldNotifyFiltersSpam(t *testing.T) {
	svc := &NotificationService{}
	settings := &domain.NotificationSettings{Enabled: true, IncludeSpam: false, OnlyPrimary: false}
	msg := &gmailclient.MessageSummary{LabelIDs: []string{"SPAM"}, Subject: "win", From: "x@y.com"}
	if svc.shouldNotify(settings, msg) {
		t.Fatal("spam should be filtered")
	}
	settings.IncludeSpam = true
	if !svc.shouldNotify(settings, msg) {
		t.Fatal("spam should pass when allowed")
	}
}

func TestShouldNotifyKeywordFilter(t *testing.T) {
	svc := &NotificationService{}
	settings := &domain.NotificationSettings{
		OnlyPrimary:   false,
		KeywordFilter: "invoice",
	}
	msg := &gmailclient.MessageSummary{LabelIDs: []string{"INBOX"}, Subject: "Hello", Snippet: "no match"}
	if svc.shouldNotify(settings, msg) {
		t.Fatal("should filter non-matching keyword")
	}
	msg.Subject = "Your invoice #12"
	if !svc.shouldNotify(settings, msg) {
		t.Fatal("should allow matching keyword")
	}
}
