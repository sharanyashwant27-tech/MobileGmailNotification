package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/yashs/mobile-gmail-notification/internal/auth"
	"github.com/yashs/mobile-gmail-notification/internal/config"
	"github.com/yashs/mobile-gmail-notification/internal/fcm"
	"github.com/yashs/mobile-gmail-notification/internal/logger"
	"github.com/yashs/mobile-gmail-notification/internal/repository"
	"github.com/yashs/mobile-gmail-notification/internal/server"
	"github.com/yashs/mobile-gmail-notification/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logger.New(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := repository.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	userRepo := repository.NewUserRepo(pool)
	gmailRepo := repository.NewGmailAccountRepo(pool)
	deviceRepo := repository.NewDeviceTokenRepo(pool)
	notifRepo := repository.NewNotificationRepo(pool)
	settingsRepo := repository.NewSettingsRepo(pool)
	refreshRepo := repository.NewRefreshTokenRepo(pool)
	oauthStateRepo := repository.NewOAuthStateRepo(pool)

	tm := auth.NewTokenManager(cfg.JWTSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	googleOAuth := auth.NewGoogleOAuth(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURI, cfg.GoogleScopes)

	authSvc := service.NewAuthService(userRepo, refreshRepo, settingsRepo, tm)
	gmailSvc := service.NewGmailAccountService(
		gmailRepo, oauthStateRepo, googleOAuth,
		cfg.TokenEncryptionKey, cfg.GmailPubSubTopic, cfg.GmailWatchLabelIDs, log,
	)
	qrRepo := repository.NewQRLoginSessionRepo(pool)
	qrSvc := service.NewQRLoginService(qrRepo, authSvc, gmailSvc, googleOAuth, cfg.BaseURL, log)

	notifier, err := fcm.New(ctx, cfg.FirebaseCredentialsFile, log)
	if err != nil {
		log.Error("fcm init failed", "error", err)
		os.Exit(1)
	}
	notifSvc := service.NewNotificationService(notifRepo, settingsRepo, deviceRepo, gmailSvc, notifier, log)

	renewer := service.NewWatchRenewer(gmailSvc, log)
	go renewer.Run(ctx)

	if poller := service.NewHistoryPoller(notifSvc, gmailSvc, log, cfg.GmailHistoryPollFallback); poller != nil {
		go poller.Run(ctx)
	}

	router := server.NewRouter(server.Dependencies{
		Config:        cfg,
		Log:           log,
		TokenManager:  tm,
		Auth:          authSvc,
		QRLogin:       qrSvc,
		Gmail:         gmailSvc,
		Notifications: notifSvc,
	})

	addr := ":" + cfg.AppPort
	if err := server.ListenAndServe(ctx, addr, router, log); err != nil {
		log.Error("server exited", "error", err)
		os.Exit(1)
	}
}
