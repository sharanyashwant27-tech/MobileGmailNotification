package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yashs/mobile-gmail-notification/internal/domain"
	gmailclient "github.com/yashs/mobile-gmail-notification/internal/gmail"
	"github.com/yashs/mobile-gmail-notification/internal/middleware"
	"github.com/yashs/mobile-gmail-notification/internal/service"
	"github.com/yashs/mobile-gmail-notification/pkg/apperrors"
	"github.com/yashs/mobile-gmail-notification/pkg/response"
)

type AuthHandler struct {
	svc *service.AuthService
	qr  *service.QRLoginService
	log *slog.Logger
}

func NewAuthHandler(svc *service.AuthService, qr *service.QRLoginService, log *slog.Logger) *AuthHandler {
	return &AuthHandler{svc: svc, qr: qr, log: log}
}

type credentialsRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, h.log, apperrors.ErrValidation)
		return
	}
	res, err := h.svc.Register(r.Context(), req.Email, req.Password, req.DisplayName)
	if err != nil {
		response.Error(w, h.log, err)
		return
	}
	response.JSON(w, http.StatusCreated, res)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, h.log, apperrors.ErrValidation)
		return
	}
	res, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		response.Error(w, h.log, err)
		return
	}
	response.JSON(w, http.StatusOK, res)
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, h.log, apperrors.ErrValidation)
		return
	}
	res, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		response.Error(w, h.log, err)
		return
	}
	response.JSON(w, http.StatusOK, res)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, h.log, apperrors.ErrUnauthorized)
		return
	}
	_ = h.svc.Logout(r.Context(), uid)
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, h.log, apperrors.ErrUnauthorized)
		return
	}
	user, err := h.svc.Me(r.Context(), uid)
	if err != nil {
		response.Error(w, h.log, err)
		return
	}
	response.JSON(w, http.StatusOK, user)
}

type profileRequest struct {
	DisplayName *string `json:"display_name"`
	DarkMode    *bool   `json:"dark_mode"`
}

func (h *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, h.log, apperrors.ErrUnauthorized)
		return
	}
	var req profileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, h.log, apperrors.ErrValidation)
		return
	}
	name := ""
	if req.DisplayName != nil {
		name = *req.DisplayName
	}
	user, err := h.svc.UpdateProfile(r.Context(), uid, name, req.DarkMode)
	if err != nil {
		response.Error(w, h.log, err)
		return
	}
	response.JSON(w, http.StatusOK, user)
}

func (h *AuthHandler) CreateQRSession(w http.ResponseWriter, r *http.Request) {
	if h.qr == nil {
		response.Error(w, h.log, apperrors.ErrInternal)
		return
	}
	res, err := h.qr.CreateSession(r.Context())
	if err != nil {
		response.Error(w, h.log, err)
		return
	}
	response.JSON(w, http.StatusCreated, res)
}

func (h *AuthHandler) StartQRSession(w http.ResponseWriter, r *http.Request) {
	if h.qr == nil {
		response.Error(w, h.log, apperrors.ErrInternal)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, h.log, apperrors.ErrValidation)
		return
	}
	authURL, err := h.qr.StartURL(r.Context(), id)
	if err != nil {
		response.Error(w, h.log, err)
		return
	}
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *AuthHandler) QRStatus(w http.ResponseWriter, r *http.Request) {
	if h.qr == nil {
		response.Error(w, h.log, apperrors.ErrInternal)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, h.log, apperrors.ErrValidation)
		return
	}
	st, err := h.qr.Status(r.Context(), id)
	if err != nil {
		response.Error(w, h.log, err)
		return
	}
	response.JSON(w, http.StatusOK, st)
}

type GmailHandler struct {
	svc *service.GmailAccountService
	qr  *service.QRLoginService
	log *slog.Logger
}

func NewGmailHandler(svc *service.GmailAccountService, qr *service.QRLoginService, log *slog.Logger) *GmailHandler {
	return &GmailHandler{svc: svc, qr: qr, log: log}
}

func (h *GmailHandler) BeginLink(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, h.log, apperrors.ErrUnauthorized)
		return
	}
	url, err := h.svc.BeginLink(r.Context(), uid)
	if err != nil {
		response.Error(w, h.log, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"authorization_url": url})
}

func (h *GmailHandler) Callback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	code := r.URL.Query().Get("code")
	if state == "" || code == "" {
		response.Error(w, h.log, apperrors.ErrOAuthFailed)
		return
	}

	if h.qr != nil {
		handled, err := h.qr.TryCompleteQR(r.Context(), state, code)
		if handled {
			if err != nil {
				response.Error(w, h.log, err)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><body style="font-family:sans-serif;text-align:center;padding:3rem">
<h1>Signed in with Gmail</h1>
<p>You can close this window and return to the computer where the QR code was shown.</p>
<script>setTimeout(function(){ window.close(); }, 1500);</script>
</body></html>`))
			return
		}
	}

	account, err := h.svc.CompleteLink(r.Context(), state, code)
	if err != nil {
		response.Error(w, h.log, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!DOCTYPE html><html><body style="font-family:sans-serif;text-align:center;padding:3rem">
<h1>Gmail connected</h1><p>Account ` + account.Email + ` linked successfully. You can return to the app.</p>
</body></html>`))
}

func (h *GmailHandler) List(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, h.log, apperrors.ErrUnauthorized)
		return
	}
	list, err := h.svc.List(r.Context(), uid)
	if err != nil {
		response.Error(w, h.log, err)
		return
	}
	response.JSON(w, http.StatusOK, list)
}

type notifyToggleRequest struct {
	Enabled bool `json:"enabled"`
}

func (h *GmailHandler) SetNotifications(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, h.log, apperrors.ErrUnauthorized)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, h.log, apperrors.ErrValidation)
		return
	}
	var req notifyToggleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, h.log, apperrors.ErrValidation)
		return
	}
	account, err := h.svc.SetNotifications(r.Context(), uid, id, req.Enabled)
	if err != nil {
		response.Error(w, h.log, err)
		return
	}
	response.JSON(w, http.StatusOK, account)
}

func (h *GmailHandler) Unlink(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, h.log, apperrors.ErrUnauthorized)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, h.log, apperrors.ErrValidation)
		return
	}
	if err := h.svc.Unlink(r.Context(), uid, id); err != nil {
		response.Error(w, h.log, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "unlinked"})
}

type NotificationHandler struct {
	svc *service.NotificationService
	log *slog.Logger
}

func NewNotificationHandler(svc *service.NotificationService, log *slog.Logger) *NotificationHandler {
	return &NotificationHandler{svc: svc, log: log}
}

func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, h.log, apperrors.ErrUnauthorized)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	list, err := h.svc.List(r.Context(), uid, limit, offset)
	if err != nil {
		response.Error(w, h.log, err)
		return
	}
	response.JSON(w, http.StatusOK, list)
}

func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, h.log, apperrors.ErrUnauthorized)
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		response.Error(w, h.log, apperrors.ErrValidation)
		return
	}
	if err := h.svc.MarkRead(r.Context(), uid, id); err != nil {
		response.Error(w, h.log, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, h.log, apperrors.ErrUnauthorized)
		return
	}
	if err := h.svc.MarkAllRead(r.Context(), uid); err != nil {
		response.Error(w, h.log, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *NotificationHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, h.log, apperrors.ErrUnauthorized)
		return
	}
	n, err := h.svc.UnreadCount(r.Context(), uid)
	if err != nil {
		response.Error(w, h.log, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]int{"unread": n})
}

func (h *NotificationHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, h.log, apperrors.ErrUnauthorized)
		return
	}
	st, err := h.svc.GetSettings(r.Context(), uid)
	if err != nil {
		response.Error(w, h.log, err)
		return
	}
	response.JSON(w, http.StatusOK, st)
}

func (h *NotificationHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, h.log, apperrors.ErrUnauthorized)
		return
	}
	var st domain.NotificationSettings
	if err := json.NewDecoder(r.Body).Decode(&st); err != nil {
		response.Error(w, h.log, apperrors.ErrValidation)
		return
	}
	out, err := h.svc.UpdateSettings(r.Context(), uid, &st)
	if err != nil {
		response.Error(w, h.log, err)
		return
	}
	response.JSON(w, http.StatusOK, out)
}

type deviceRequest struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

func (h *NotificationHandler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, h.log, apperrors.ErrUnauthorized)
		return
	}
	var req deviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, h.log, apperrors.ErrValidation)
		return
	}
	if err := h.svc.RegisterDevice(r.Context(), uid, req.Token, req.Platform); err != nil {
		response.Error(w, h.log, err)
		return
	}
	response.JSON(w, http.StatusOK, map[string]string{"status": "registered"})
}

func (h *NotificationHandler) UnregisterDevice(w http.ResponseWriter, r *http.Request) {
	uid, ok := middleware.UserIDFromContext(r.Context())
	if !ok {
		response.Error(w, h.log, apperrors.ErrUnauthorized)
		return
	}
	var req deviceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, h.log, apperrors.ErrValidation)
		return
	}
	_ = h.svc.UnregisterDevice(r.Context(), uid, req.Token)
	response.JSON(w, http.StatusOK, map[string]string{"status": "unregistered"})
}

// PubSubHandler receives Gmail push notifications via Google Cloud Pub/Sub push subscriptions.
type PubSubHandler struct {
	svc *service.NotificationService
	log *slog.Logger
}

func NewPubSubHandler(svc *service.NotificationService, log *slog.Logger) *PubSubHandler {
	return &PubSubHandler{svc: svc, log: log}
}

type pubSubPushEnvelope struct {
	Message struct {
		Data      string `json:"data"`
		MessageID string `json:"messageId"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

func (h *PubSubHandler) Receive(w http.ResponseWriter, r *http.Request) {
	var env pubSubPushEnvelope
	if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
		response.Error(w, h.log, apperrors.ErrValidation)
		return
	}
	n, err := gmailclient.DecodePubSubData(env.Message.Data)
	if err != nil {
		h.log.Warn("invalid pubsub payload", "error", err)
		// Ack to avoid infinite retries on poison messages.
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := h.svc.HandlePubSub(r.Context(), n.EmailAddress, n.HistoryID); err != nil {
		h.log.Error("pubsub handling failed", "error", err)
		http.Error(w, "processing failed", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
