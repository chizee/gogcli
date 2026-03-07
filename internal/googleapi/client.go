package googleapi

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/option"

	"github.com/steipete/gogcli/internal/googleauth"
)

const (
	// responseHeaderTimeout limits the time waiting for the server to begin
	// responding (send response headers). Once headers arrive and the body
	// starts streaming, there is no hard cap — large file downloads are not
	// cut short. This replaces the former http.Client.Timeout which applied
	// to the entire request lifecycle and caused timeouts on large Drive
	// file downloads.
	responseHeaderTimeout = 30 * time.Second

	// tokenExchangeTimeout is applied to the short-lived HTTP client used
	// for OAuth2 token refresh exchanges, which should always be fast.
	tokenExchangeTimeout = 30 * time.Second
)

func optionsForAccount(ctx context.Context, service googleauth.Service, email string) ([]option.ClientOption, error) {
	scopes, err := googleauth.Scopes(service)
	if err != nil {
		return nil, fmt.Errorf("resolve scopes: %w", err)
	}

	return optionsForAccountScopes(ctx, string(service), email, scopes)
}

func optionsForAccountScopes(ctx context.Context, serviceLabel string, email string, scopes []string) ([]option.ClientOption, error) {
	slog.Debug("creating client options with custom scopes", "serviceLabel", serviceLabel, "email", email)

	ts, err := tokenSourceForAvailableAccountAuth(ctx, serviceLabel, email, scopes)
	if err != nil {
		return nil, err
	}

	baseTransport := newBaseTransport()
	retryTransport := NewRetryTransport(&oauth2.Transport{
		Source: ts,
		Base:   baseTransport,
	})
	c := &http.Client{
		Transport: retryTransport,
		// No Timeout set: large file downloads (Drive videos, etc.) must not
		// be cut short. Server responsiveness is guarded by the transport's
		// ResponseHeaderTimeout instead.
	}

	slog.Debug("client options with custom scopes created successfully", "serviceLabel", serviceLabel, "email", email)

	return []option.ClientOption{option.WithHTTPClient(c)}, nil
}

func newBaseTransport() *http.Transport {
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok || defaultTransport == nil {
		return &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
			ResponseHeaderTimeout: responseHeaderTimeout,
		}
	}

	// Clone() deep-copies TLSClientConfig, so no additional clone needed.
	transport := defaultTransport.Clone()
	transport.ResponseHeaderTimeout = responseHeaderTimeout

	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
		return transport
	}

	if transport.TLSClientConfig.MinVersion < tls.VersionTLS12 {
		transport.TLSClientConfig.MinVersion = tls.VersionTLS12
	}

	return transport
}
