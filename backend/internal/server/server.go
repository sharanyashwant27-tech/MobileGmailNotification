package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/yashs/mobile-gmail-notification/internal/auth"
	"github.com/yashs/mobile-gmail-notification/internal/config"
	"github.com/yashs/mobile-gmail-notification/internal/handler"
	"github.com/yashs/mobile-gmail-notification/internal/middleware"
	"github.com/yashs/mobile-gmail-notification/internal/service"
)

// Dependencies groups wired services for the HTTP server.
type Dependencies struct {
	Config        *config.Config
	Log           *slog.Logger
	TokenManager  *auth.TokenManager
	Auth          *service.AuthService
	QRLogin       *service.QRLoginService
	Gmail         *service.GmailAccountService
	Notifications *service.NotificationService
}

// NewRouter builds the chi router with all REST endpoints.
func NewRouter(deps Dependencies) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recover(deps.Log))
	r.Use(middleware.Logger(deps.Log))
	r.Use(middleware.CORS(deps.Config.CORSAllowedOrigins))
	r.Use(middleware.RateLimit(deps.Config.RateLimitRPS, deps.Config.RateLimitBurst))

	authH := handler.NewAuthHandler(deps.Auth, deps.QRLogin, deps.Log)
	gmailH := handler.NewGmailHandler(deps.Gmail, deps.QRLogin, deps.Log)
	notifH := handler.NewNotificationHandler(deps.Notifications, deps.Log)
	pubsubH := handler.NewPubSubHandler(deps.Notifications, deps.Log)

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mountWebUI(r)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", authH.Register)
		r.Post("/auth/login", authH.Login)
		r.Post("/auth/refresh", authH.Refresh)

		// Gmail QR login (desktop polls; phone scans and completes Google OAuth).
		r.Post("/auth/qr/session", authH.CreateQRSession)
		r.Get("/auth/qr/{id}/start", authH.StartQRSession)
		r.Get("/auth/qr/{id}/status", authH.QRStatus)

		// Google OAuth callback (browser redirect; CSRF via state param).
		r.Get("/oauth/google/callback", gmailH.Callback)

		// Gmail Pub/Sub push endpoint (secured via network/IAM in production).
		r.Post("/webhooks/gmail-pubsub", pubsubH.Receive)

		r.Group(func(r chi.Router) {
			r.Use(middleware.JWTAuth(deps.TokenManager, deps.Log))

			r.Post("/auth/logout", authH.Logout)
			r.Get("/auth/me", authH.Me)
			r.Patch("/auth/me", authH.UpdateProfile)

			r.Post("/gmail/accounts/link", gmailH.BeginLink)
			r.Get("/gmail/accounts", gmailH.List)
			r.Patch("/gmail/accounts/{id}/notifications", gmailH.SetNotifications)
			r.Delete("/gmail/accounts/{id}", gmailH.Unlink)

			r.Get("/notifications", notifH.List)
			r.Post("/notifications/read-all", notifH.MarkAllRead)
			r.Post("/notifications/{id}/read", notifH.MarkRead)
			r.Get("/notifications/unread-count", notifH.UnreadCount)

			r.Get("/settings/notifications", notifH.GetSettings)
			r.Put("/settings/notifications", notifH.UpdateSettings)

			r.Post("/devices", notifH.RegisterDevice)
			r.Delete("/devices", notifH.UnregisterDevice)
		})
	})

	return r
}

// ListenAndServe starts the HTTP server with graceful shutdown support.
func ListenAndServe(ctx context.Context, addr string, handler http.Handler, log *slog.Logger) error {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("server listening", "addr", addr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		log.Info("shutting down server")
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

func mountWebUI(r chi.Router) {
	webRoot := resolveWebRoot()
	staticDir := filepath.Join(webRoot, "static")
	indexPath := filepath.Join(webRoot, "index.html")

	if _, err := os.Stat(indexPath); err != nil {
		return
	}

	fileServer := http.FileServer(http.Dir(staticDir))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		http.ServeFile(w, req, indexPath)
	})

	// SPA-friendly fallback for non-API paths when index exists.
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/api/") || req.URL.Path == "/healthz" {
			http.NotFound(w, req)
			return
		}
		if strings.HasPrefix(req.URL.Path, "/static/") {
			http.NotFound(w, req)
			return
		}
		http.ServeFile(w, req, indexPath)
	})
}

func resolveWebRoot() string {
	candidates := []string{
		"web",
		filepath.Join(".", "web"),
		"/app/web",
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "web"))
	}
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "index.html")); err == nil && !st.IsDir() {
			return c
		}
	}
	return "web"
}
