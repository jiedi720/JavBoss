package avdanyu

import (
	"encoding/json"
	"testing"
)

func TestToWindowsPath(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/mnt/e/Agent/foo/fetch.mjs", `E:\Agent\foo\fetch.mjs`},
		{"/mnt/c/Users/test/fetch.mjs", `C:\Users\test\fetch.mjs`},
		{"/mnt/D/lower/fetch.mjs", `D:\lower\fetch.mjs`},
		{`/already/windows\path.mjs`, `/already/windows\path.mjs`},
		{"/mnt/notadrive/foo", "/mnt/notadrive/foo"},
	}
	for _, tc := range cases {
		if got := toWindowsPath(tc.in); got != tc.want {
			t.Errorf("toWindowsPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseFetchResult(t *testing.T) {
	// Mirrors the exact JSON shape emitted by fetch.mjs.
	raw := `{"keyword":"LUXU-1905","works":[{"title":"t","code":"259LUXU-1905","studio":"s","series":"se","duration_min":71,"release_date":"2026-11-11","actors":["マッスル澤野"]}],"title":"search"}`
	var result Result
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Keyword != "LUXU-1905" {
		t.Errorf("keyword = %q", result.Keyword)
	}
	if len(result.Works) != 1 {
		t.Fatalf("works len = %d", len(result.Works))
	}
	w := result.Works[0]
	if w.Code != "259LUXU-1905" || w.DurationMin != 71 || w.ReleaseDate != "2026-11-11" {
		t.Errorf("work fields mismatch: %+v", w)
	}
	if len(w.Actors) != 1 || w.Actors[0] != "マッスル澤野" {
		t.Errorf("actors = %v", w.Actors)
	}
}

func TestParseFetchResultEmptyWorks(t *testing.T) {
	// IPX-228 style result: no entries found on the site.
	raw := `{"keyword":"IPX-228","works":[],"title":"search"}`
	var result Result
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Works == nil {
		t.Error("works should decode to an empty (non-nil) slice")
	}
	if len(result.Works) != 0 {
		t.Errorf("works len = %d, want 0", len(result.Works))
	}
}

func TestParseFetchResultCloudflareBlocked(t *testing.T) {
	raw := `{"keyword":"X","works":[],"error":"cloudflare_blocked","title":"Just a moment..."}`
	var result Result
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Error != "cloudflare_blocked" {
		t.Errorf("error = %q", result.Error)
	}
}

func TestNewClientDefaults(t *testing.T) {
	c := NewClient()
	if c.CDPBase == "" {
		t.Error("CDPBase should have a default")
	}
	if c.ScriptPath == "" {
		t.Error("ScriptPath should have a default")
	}
	if c.NodeBin == "" {
		t.Error("NodeBin should have a default")
	}
	if c.Timeout <= 0 {
		t.Error("Timeout should be positive")
	}
}
