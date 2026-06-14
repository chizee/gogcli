package cmd

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/idtoken"

	"github.com/steipete/gogcli/internal/gmailwatch"
)

var errNoNewMessages = gmailwatch.ErrNoNewMessages

const (
	gmailWatchStatusHTTPError = gmailwatch.DeliveryStatusHTTPError
	gmailWatchStatusRateLimit = gmailwatch.DeliveryStatusRateLimit
)

type gmailWatchRateLimitError = gmailwatch.RateLimitError

type gmailWatchServer struct {
	cfg             gmailWatchServeConfig
	store           *gmailWatchStore
	validator       *idtoken.Validator
	newService      func(context.Context, string) (*gmail.Service, error)
	sleep           func(context.Context, time.Duration) error
	hookClient      *http.Client
	excludeLabelIDs map[string]struct{}
	logf            func(string, ...any)
	warnf           func(string, ...any)
	now             func() time.Time
}

func (s *gmailWatchServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	handler := gmailwatch.HTTPHandler{
		Config: gmailwatch.HTTPConfig{
			Path:        s.cfg.Path,
			Account:     s.cfg.Account,
			BodyLimit:   defaultPushBodyLimitBytes,
			HasHook:     s.cfg.HookURL != "",
			AllowNoHook: s.cfg.AllowNoHook,
		},
		Authorize: s.authorize,
		Process: func(ctx context.Context, notification gmailwatch.Notification) (*gmailwatch.ProcessedPayload, error) {
			return s.watchProcessor().Process(ctx, notification)
		},
		Now:   s.currentTime,
		Warnf: s.warnf,
	}
	handler.ServeHTTP(w, r)
}

func (s *gmailWatchServer) authorize(r *http.Request) bool {
	if s.cfg.VerifyOIDC {
		bearer := bearerToken(r)
		if bearer != "" {
			if ok, err := verifyOIDCToken(r.Context(), s.validator, bearer, s.oidcAudience(r), s.cfg.OIDCEmail); ok {
				return true
			} else if err != nil {
				s.warnf("watch: oidc verify failed: %v", err)
			}
		}
		if s.cfg.SharedToken != "" {
			return sharedTokenMatches(r, s.cfg.SharedToken)
		}
		return false
	}
	if s.cfg.SharedToken == "" {
		return true
	}
	return sharedTokenMatches(r, s.cfg.SharedToken)
}

func (s *gmailWatchServer) oidcAudience(r *http.Request) string {
	if s.cfg.OIDCAudience != "" {
		return s.cfg.OIDCAudience
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if xf := r.Header.Get("X-Forwarded-Proto"); xf != "" {
		parts := strings.Split(xf, ",")
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			scheme = strings.TrimSpace(parts[0])
		}
	}
	host := r.Host
	if xf := r.Header.Get("X-Forwarded-Host"); xf != "" {
		parts := strings.Split(xf, ",")
		if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
			host = strings.TrimSpace(parts[0])
		}
	}
	return fmt.Sprintf("%s://%s%s", scheme, host, r.URL.Path)
}

func (s *gmailWatchServer) sendHook(ctx context.Context, payload *gmailHookPayload) error {
	delivery := s.deliverHook(ctx, payload)
	if delivery.Record {
		_ = s.store.RecordDelivery(delivery.Status, delivery.Note, s.currentTime())
	}

	return delivery.Err
}

func (s *gmailWatchServer) deliverHook(ctx context.Context, payload *gmailHookPayload) gmailwatch.DeliveryResult {
	data, err := json.Marshal(payload)
	if err != nil {
		return gmailwatch.DeliveryResult{Err: err}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.HookURL, bytes.NewReader(data))
	if err != nil {
		return gmailwatch.DeliveryResult{Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	if s.cfg.HookToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.cfg.HookToken)
	}
	resp, err := s.hookClient.Do(req)
	if err != nil {
		return gmailwatch.DeliveryResult{
			Status: gmailwatch.DeliveryStatusError,
			Note:   err.Error(),
			Err:    err,
			Record: true,
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		note := fmt.Sprintf("status %d", resp.StatusCode)

		return gmailwatch.DeliveryResult{
			Status: gmailwatch.DeliveryStatusHTTPError,
			Note:   note,
			Err:    fmt.Errorf("hook %s", note),
			Record: true,
		}
	}

	return gmailwatch.DeliveryResult{
		Status: gmailwatch.DeliveryStatusOK,
		Record: true,
	}
}

func bearerToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 {
		return ""
	}
	if !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func sharedTokenMatches(r *http.Request, expected string) bool {
	if expected == "" {
		return false
	}
	token := r.Header.Get("x-gog-token")
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	if token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}

func verifyOIDCToken(ctx context.Context, validator *idtoken.Validator, token, audience, expectedEmail string) (bool, error) {
	if validator == nil {
		return false, errors.New("no OIDC validator")
	}
	payload, err := validator.Validate(ctx, token, audience)
	if err != nil {
		return false, err
	}
	if expectedEmail == "" {
		return true, nil
	}
	email, _ := payload.Claims["email"].(string)
	if !strings.EqualFold(email, expectedEmail) {
		return false, fmt.Errorf("oidc email mismatch: %s", email)
	}
	return true, nil
}

func (s *gmailWatchServer) currentTime() time.Time {
	if s.now != nil {
		return s.now()
	}

	return time.Now()
}

func isStaleHistoryError(err error) bool {
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		if gerr.Code == http.StatusBadRequest || gerr.Code == http.StatusNotFound {
			msg := strings.ToLower(gerr.Message)
			if strings.Contains(msg, "history") {
				return true
			}
			for _, item := range gerr.Errors {
				if strings.Contains(strings.ToLower(item.Message), "history") {
					return true
				}
				if gerr.Code == http.StatusNotFound && strings.EqualFold(strings.TrimSpace(item.Reason), "notfound") {
					return true
				}
			}
			if gerr.Code == http.StatusNotFound && strings.Contains(msg, "not found") {
				return true
			}
		}
	}
	return strings.Contains(strings.ToLower(err.Error()), "history")
}

func isNotFoundAPIError(err error) bool {
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		return gerr.Code == http.StatusNotFound
	}
	return false
}

func gmailWatchRateLimitUntil(err error, now time.Time) (time.Time, bool) {
	var gerr *googleapi.Error
	if !errors.As(err, &gerr) || gerr.Code != http.StatusTooManyRequests {
		return time.Time{}, false
	}
	if until, ok := parseRetryAfterUntil(gerr.Header.Get("Retry-After"), now); ok {
		return until, true
	}
	return now.Add(time.Minute), true
}

func parseRetryAfterUntil(raw string, now time.Time) (time.Time, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return time.Time{}, false
	}
	if seconds, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
		if seconds < 0 {
			seconds = 0
		}
		return now.Add(time.Duration(seconds) * time.Second), true
	}
	if parsed, err := http.ParseTime(trimmed); err == nil {
		return parsed, true
	}
	if parsed, err := time.Parse(time.RFC3339, trimmed); err == nil {
		return parsed, true
	}
	return time.Time{}, false
}
