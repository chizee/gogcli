package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"github.com/steipete/gogcli/internal/ui"
)

func TestGmailSettings_TextPaths(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.Contains(path, "/gmail/v1/users/me/settings/delegates") && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(path, "/delegates/") {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"delegateEmail":      "d@b.com",
					"verificationStatus": "accepted",
					"delegationEnabled":  true,
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"delegates": []map[string]any{
					{"delegateEmail": "d@b.com", "verificationStatus": "accepted"},
				},
			})
			return
		case strings.Contains(path, "/gmail/v1/users/me/settings/delegates") && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"delegateEmail": "d@b.com", "verificationStatus": "pending"})
			return
		case strings.Contains(path, "/gmail/v1/users/me/settings/delegates/") && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
			return

		case strings.Contains(path, "/gmail/v1/users/me/settings/forwardingAddresses") && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			if strings.Contains(path, "/forwardingAddresses/") {
				_ = json.NewEncoder(w).Encode(map[string]any{"forwardingEmail": "f@b.com", "verificationStatus": "accepted"})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"forwardingAddresses": []map[string]any{
					{"forwardingEmail": "f@b.com", "verificationStatus": "accepted"},
				},
			})
			return
		case strings.Contains(path, "/gmail/v1/users/me/settings/forwardingAddresses") && r.Method == http.MethodPost:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"forwardingEmail": "f@b.com", "verificationStatus": "pending"})
			return
		case strings.Contains(path, "/gmail/v1/users/me/settings/forwardingAddresses/") && r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
			return

		case strings.Contains(path, "/gmail/v1/users/me/settings/vacation") && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"enableAutoReply":       false,
				"responseSubject":       "S",
				"responseBodyHtml":      "<b>hi</b>",
				"responseBodyPlainText": "hi",
				"startTime":             "111",
				"endTime":               "222",
				"restrictToContacts":    true,
				"restrictToDomain":      true,
			})
			return
		case strings.Contains(path, "/gmail/v1/users/me/settings/vacation") && r.Method == http.MethodPut:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"enableAutoReply":    true,
				"responseSubject":    "S2",
				"startTime":          "123",
				"endTime":            "456",
				"restrictToContacts": true,
				"restrictToDomain":   false,
			})
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer srv.Close()

	svc, err := gmail.NewService(context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	newGmailService = func(context.Context, string) (*gmail.Service, error) { return svc, nil }

	flags := &RootFlags{Account: "a@b.com", Force: true}

	u, uiErr := ui.New(ui.Options{Stdout: io.Discard, Stderr: io.Discard, Color: "never"})
	if uiErr != nil {
		t.Fatalf("ui.New: %v", uiErr)
	}
	ctx := ui.WithUI(context.Background(), u)

	if err := runKong(t, &GmailDelegatesListCmd{}, []string{}, ctx, flags); err != nil {
		t.Fatalf("delegates list: %v", err)
	}

	if err := runKong(t, &GmailDelegatesGetCmd{}, []string{"d@b.com"}, ctx, flags); err != nil {
		t.Fatalf("delegates get: %v", err)
	}

	if err := runKong(t, &GmailDelegatesAddCmd{}, []string{"d@b.com"}, ctx, flags); err != nil {
		t.Fatalf("delegates add: %v", err)
	}

	if err := runKong(t, &GmailDelegatesRemoveCmd{}, []string{"d@b.com"}, ctx, flags); err != nil {
		t.Fatalf("delegates remove: %v", err)
	}

	if err := runKong(t, &GmailForwardingListCmd{}, []string{}, ctx, flags); err != nil {
		t.Fatalf("forwarding list: %v", err)
	}

	if err := runKong(t, &GmailForwardingGetCmd{}, []string{"f@b.com"}, ctx, flags); err != nil {
		t.Fatalf("forwarding get: %v", err)
	}

	if err := runKong(t, &GmailForwardingCreateCmd{}, []string{"f@b.com"}, ctx, flags); err != nil {
		t.Fatalf("forwarding create: %v", err)
	}

	if err := runKong(t, &GmailForwardingDeleteCmd{}, []string{"f@b.com"}, ctx, flags); err != nil {
		t.Fatalf("forwarding delete: %v", err)
	}

	if err := runKong(t, &GmailVacationGetCmd{}, []string{}, ctx, flags); err != nil {
		t.Fatalf("vacation get: %v", err)
	}

	if err := runKong(t, &GmailVacationUpdateCmd{}, []string{
		"--enable",
		"--subject", "S2",
		"--body", "<b>hi</b>",
		"--start", "2025-01-01T00:00:00Z",
		"--end", "2025-01-02T00:00:00Z",
		"--contacts-only",
	}, ctx, flags); err != nil {
		t.Fatalf("vacation update: %v", err)
	}
}

func TestGmailSettings_JSONEmptyListsUseArrays(t *testing.T) {
	origNew := newGmailService
	t.Cleanup(func() { newGmailService = origNew })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/gmail/v1/users/me/settings/forwardingAddresses"):
		case strings.Contains(r.URL.Path, "/gmail/v1/users/me/settings/delegates"):
		case strings.Contains(r.URL.Path, "/gmail/v1/users/me/settings/sendAs"):
		default:
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer srv.Close()

	svc, err := gmail.NewService(context.Background(),
		option.WithoutAuthentication(),
		option.WithHTTPClient(srv.Client()),
		option.WithEndpoint(srv.URL+"/"),
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	newGmailService = func(context.Context, string) (*gmail.Service, error) { return svc, nil }

	tests := []struct {
		name string
		run  func(context.Context, *RootFlags) error
		key  string
	}{
		{
			name: "forwarding",
			run: func(ctx context.Context, flags *RootFlags) error {
				return runKong(t, &GmailForwardingListCmd{}, []string{}, ctx, flags)
			},
			key: "forwardingAddresses",
		},
		{
			name: "delegates",
			run: func(ctx context.Context, flags *RootFlags) error {
				return runKong(t, &GmailDelegatesListCmd{}, []string{}, ctx, flags)
			},
			key: "delegates",
		},
		{
			name: "sendAs",
			run: func(ctx context.Context, flags *RootFlags) error {
				return runKong(t, &GmailSendAsListCmd{}, []string{}, ctx, flags)
			},
			key: "sendAs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var runErr error
			stdout := captureStdout(t, func() {
				runErr = tt.run(newCmdJSONContext(t), &RootFlags{Account: "a@b.com", JSON: true})
			})
			if runErr != nil {
				t.Fatalf("run: %v", runErr)
			}
			var got map[string][]json.RawMessage
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatalf("json output %q: %v", stdout, err)
			}
			items, ok := got[tt.key]
			if !ok {
				t.Fatalf("missing key %q in %s", tt.key, stdout)
			}
			if items == nil {
				t.Fatalf("%s is nil in %s", tt.key, stdout)
			}
			if len(items) != 0 {
				t.Fatalf("%s len = %d in %s", tt.key, len(items), stdout)
			}
		})
	}
}

func TestGmailVacationUpdate_EnableDisableConflict(t *testing.T) {
	flags := &RootFlags{Account: "a@b.com"}
	err := runKong(t, &GmailVacationUpdateCmd{}, []string{"--enable", "--disable"}, context.Background(), flags)
	if err == nil {
		t.Fatalf("expected conflict error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("expected usage exit code 2, got %d (err=%v)", got, err)
	}
}

func TestGmailVacationUpdate_InvalidTimesAreUsageErrors(t *testing.T) {
	flags := &RootFlags{Account: "a@b.com", DryRun: true}
	for _, tt := range []struct {
		name string
		args []string
	}{
		{name: "start", args: []string{"--start", "nope"}},
		{name: "end", args: []string{"--end", "nope"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := runKong(t, &GmailVacationUpdateCmd{}, tt.args, context.Background(), flags)
			if err == nil {
				t.Fatal("expected error")
			}
			if got := ExitCode(err); got != 2 {
				t.Fatalf("expected usage exit code 2, got %d (err=%v)", got, err)
			}
		})
	}
}
