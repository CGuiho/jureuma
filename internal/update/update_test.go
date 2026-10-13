package update

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCompare(t *testing.T) {
	order := []string{"0.9.0", "1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.2.3", "v1.10.0"}
	for i := 0; i+1 < len(order); i++ {
		a, ok1 := Parse(order[i])
		b, ok2 := Parse(order[i+1])
		if !ok1 || !ok2 || Compare(a, b) >= 0 || Compare(b, a) <= 0 {
			t.Errorf("%s < %s failed", order[i], order[i+1])
		}
	}
	for _, bad := range []string{"", "1.2", "01.2.3", "1.2.3-", "x.y.z", "1.2.3-a..b"} {
		if _, ok := Parse(bad); ok {
			t.Errorf("%q accepted", bad)
		}
	}
	if v, _ := Parse("1.2.3-canary.4"); v.Channel() != "canary" {
		t.Error("channel")
	}
}

func TestReinstallCommand(t *testing.T) {
	if got := ReinstallCommand("linux", "1.2.3", ""); got != "curl -fsSL "+InstallSh+" | sh -s -- --version 1.2.3" {
		t.Error(got)
	}
	if got := ReinstallCommand("darwin", "", "beta"); !strings.HasSuffix(got, "--channel beta") {
		t.Error(got)
	}
	if got := ReinstallCommand("windows", "1.2.3", ""); !strings.Contains(got, "-Version '1.2.3'") || !strings.HasPrefix(got, "powershell.exe") {
		t.Error(got)
	}
}

func TestCheckCacheNoticeAndLease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a := fullAssets()
		fmt.Fprintf(w, `[{"tag_name":"v3.0.0","draft":false,"assets":[{"name":"nothing.txt"}]},{"tag_name":"v2.0.0-beta.1","draft":false,"assets":%s},{"tag_name":"v1.5.0","draft":false,"assets":%s},{"tag_name":"v9.9.9","draft":true,"assets":%s},{"tag_name":"junk"}]`, a, a, a)
	}))
	defer srv.Close()
	t.Setenv(ReleasesEnv, srv.URL)
	home := t.TempDir()
	if !NeedsCheck(home) || Notice(home, "1.0.0") != "" {
		t.Fatal("missing cache must be a miss without notice")
	}
	c, err := Check(context.Background(), home, "1.0.0")
	if err != nil || c.LatestVersion != "1.5.0" || !c.NewVersionAvailable {
		t.Fatalf("%+v %v", c, err)
	}
	if n := Notice(home, "1.0.0"); n != "⚠ New version available: v1.5.0\n  run jureuma upgrade to update\n" {
		t.Fatalf("notice %q", n)
	}
	if Notice(home, "1.5.0") != "" || Notice(home, "2.0.0") != "" {
		t.Fatal("no notice for equal or newer")
	}
	os.WriteFile(filepath.Join(home, CacheFile), []byte("{corrupt"), 0o600)
	if Notice(home, "1.0.0") != "" || !NeedsCheck(home) {
		t.Fatal("corrupt cache must be a silent miss")
	}
	tok, ok := AcquireLease(home)
	if !ok {
		t.Fatal("lease")
	}
	if _, ok := AcquireLease(home); ok {
		t.Fatal("second lease must be refused")
	}
	old := time.Now().Add(-2 * MaxLeaseAge)
	os.Chtimes(filepath.Join(home, LeaseFile), old, old)
	tok2, ok := AcquireLease(home)
	if !ok {
		t.Fatal("abandoned lease must be broken")
	}
	ReleaseLease(home, tok)
	if _, err := os.Stat(filepath.Join(home, LeaseFile)); err != nil {
		t.Fatal("foreign token must not release the lease")
	}
	ReleaseLease(home, tok2)
	if _, err := os.Stat(filepath.Join(home, LeaseFile)); err == nil {
		t.Fatal("lease not released")
	}
	rs, _ := FetchReleases(context.Background())
	if r, ok := Select(rs, "", "beta"); !ok || r.Version != "2.0.0-beta.1" {
		t.Fatal("channel select")
	}
	if _, ok := Select(rs, "9.9.9", ""); ok {
		t.Fatal("draft selected")
	}
}

// fullAssets returns a GitHub assets array with every required asset for
// the running platform.
func fullAssets() string {
	var parts []string
	for _, a := range RequiredAssets(runtime.GOOS, runtime.GOARCH) {
		parts = append(parts, `{"name":"`+a+`"}`)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func TestSelectSkipsIncompleteReleases(t *testing.T) {
	full := RequiredAssets(runtime.GOOS, runtime.GOARCH)
	rs := []Release{
		{Version: "9.9.9", Channel: "stable", Assets: []string{"nothing.txt"}},
		{Version: "2.0.0", Channel: "stable", Assets: full[:len(full)-1]},
		{Version: "1.0.0", Channel: "stable", Assets: full},
	}
	if r, ok := Select(rs, "", ""); !ok || r.Version != "1.0.0" {
		t.Fatalf("got %+v %v, want 1.0.0", r, ok)
	}
	if _, ok := Select(rs, "9.9.9", ""); ok {
		t.Fatal("an exact version without assets must not be selected")
	}
	if got := Installable(rs); len(got) != 1 {
		t.Fatalf("Installable = %+v", got)
	}
}
