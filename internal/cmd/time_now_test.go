package cmd

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/steipete/gogcli/internal/outfmt"
	"github.com/steipete/gogcli/internal/ui"
)

func TestTimeNowCmd_JSON(t *testing.T) {
	u, err := ui.New(ui.Options{Stdout: os.Stdout, Stderr: os.Stderr, Color: "never"})
	if err != nil {
		t.Fatalf("ui.New: %v", err)
	}
	ctx := outfmt.WithMode(ui.WithUI(context.Background(), u), outfmt.Mode{JSON: true})

	out := captureStdout(t, func() {
		if err := runKong(t, &TimeNowCmd{}, []string{"--timezone", "UTC"}, ctx, &RootFlags{}); err != nil {
			t.Fatalf("runKong: %v", err)
		}
	})

	var parsed struct {
		Timezone    string `json:"timezone"`
		UTCOffset   string `json:"utc_offset"`
		CurrentTime string `json:"current_time"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("json parse: %v", err)
	}
	if parsed.Timezone != "UTC" {
		t.Fatalf("unexpected timezone: %q", parsed.Timezone)
	}
	if parsed.UTCOffset != "+00:00" {
		t.Fatalf("unexpected offset: %q", parsed.UTCOffset)
	}
	if parsed.CurrentTime == "" {
		t.Fatalf("expected current_time")
	}
}

func TestTimeNowCmd_InvalidTimezone(t *testing.T) {
	err := runKong(t, &TimeNowCmd{}, []string{"--timezone", "Nope/Zone"}, context.Background(), &RootFlags{})
	if err == nil {
		t.Fatalf("expected error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("expected usage exit code 2, got %d (err=%v)", got, err)
	}
}
