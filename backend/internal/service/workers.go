package service

import (
	"context"
	"log/slog"
	"time"
)

// WatchRenewer periodically renews Gmail users.watch subscriptions (max ~7 days).
type WatchRenewer struct {
	accounts *GmailAccountService
	log      *slog.Logger
	interval time.Duration
	skew     time.Duration
}

func NewWatchRenewer(accounts *GmailAccountService, log *slog.Logger) *WatchRenewer {
	return &WatchRenewer{
		accounts: accounts,
		log:      log,
		interval: time.Hour,
		skew:     6 * time.Hour,
	}
}

func (w *WatchRenewer) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	w.renewDue(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.renewDue(ctx)
		}
	}
}

func (w *WatchRenewer) renewDue(ctx context.Context) {
	list, err := w.accounts.ListActive(ctx)
	if err != nil {
		w.log.Error("list accounts for watch renew", "error", err)
		return
	}
	deadline := time.Now().UTC().Add(w.skew)
	for _, a := range list {
		if a.WatchExpiration == nil || a.WatchExpiration.Before(deadline) {
			if err := w.accounts.RenewWatch(ctx, a); err != nil {
				w.log.Warn("renew watch failed", "email", a.Email, "error", err)
			} else {
				w.log.Info("renewed gmail watch", "email", a.Email)
			}
		}
	}
}

// HistoryPoller is an optional fallback that polls Gmail history when Pub/Sub is unavailable.
type HistoryPoller struct {
	notifications *NotificationService
	accounts      *GmailAccountService
	log           *slog.Logger
	interval      time.Duration
}

func NewHistoryPoller(notifications *NotificationService, accounts *GmailAccountService, log *slog.Logger, enabled bool) *HistoryPoller {
	if !enabled {
		return nil
	}
	return &HistoryPoller{
		notifications: notifications,
		accounts:      accounts,
		log:           log,
		interval:      2 * time.Minute,
	}
}

func (p *HistoryPoller) Run(ctx context.Context) {
	if p == nil {
		return
	}
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.poll(ctx)
		}
	}
}

func (p *HistoryPoller) poll(ctx context.Context) {
	list, err := p.accounts.ListActive(ctx)
	if err != nil {
		p.log.Error("history poll list accounts", "error", err)
		return
	}
	for _, a := range list {
		if err := p.notifications.SyncAccount(ctx, a); err != nil {
			p.log.Warn("history poll sync failed", "email", a.Email, "error", err)
		}
	}
}
