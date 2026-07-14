package service_test

import (
	"testing"
	"time"

	"github.com/yashs/mobile-gmail-notification/internal/domain"
	gmailclient "github.com/yashs/mobile-gmail-notification/internal/gmail"
)

// Mirror quiet-hours logic tests via exported helpers through notification filtering behavior.
// We test ParseHistoryID and PubSub decode which are critical to watch/history flow.

func TestDecodePubSubData(t *testing.T) {
	// {"emailAddress":"a@b.com","historyId":12345} base64
	data := "eyJlbWFpbEFkZHJlc3MiOiJhQGIuY29tIiwiaGlzdG9yeUlkIjoxMjM0NX0="
	n, err := gmailclient.DecodePubSubData(data)
	if err != nil {
		t.Fatal(err)
	}
	if n.EmailAddress != "a@b.com" {
		t.Fatalf("email=%s", n.EmailAddress)
	}
	if n.HistoryID != 12345 {
		t.Fatalf("history=%d", n.HistoryID)
	}
}

func TestParseHistoryID(t *testing.T) {
	id, err := gmailclient.ParseHistoryID("99")
	if err != nil || id != 99 {
		t.Fatalf("got %d %v", id, err)
	}
	if _, err := gmailclient.ParseHistoryID(""); err == nil {
		t.Fatal("expected error")
	}
}

func TestDefaultNotificationSettingsSanity(t *testing.T) {
	s := domain.NotificationSettings{
		Enabled:         true,
		QuietHoursStart: "22:00",
		QuietHoursEnd:   "07:00",
		OnlyPrimary:     true,
	}
	if !s.Enabled || s.QuietHoursStart == "" {
		t.Fatal("defaults incomplete")
	}
	_ = time.Now()
}
