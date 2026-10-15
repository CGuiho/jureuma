package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
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

// catalogEntry renders one release-catalog entry.
func catalogEntry(tag string, draft bool) string {
	return fmt.Sprintf(`{"tag_name":%q,"draft":%t,"html_url":"x","assets":[]}`, tag, draft)
}

// catalogPage renders a release-catalog page from rendered entries.
func catalogPage(entries ...string) string {
	return "[" + strings.Join(entries, ",") + "]"
}

// catalogEntries returns n entries with sequential SemVer tags starting at first.
func catalogEntries(first, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = catalogEntry(fmt.Sprintf("v%d.0.0", first+i), false)
	}
	return out
}

func TestFetchReleasesPaginatesToShortPage(t *testing.T) {
	t.Setenv("JUREUMA_USER_HOME", t.TempDir())
	// Page 1 is full (100 entries) and mixes valid releases with one draft and
	// one malformed tag; page 2 is short and must stop the walk. Any request
	// beyond page 2 fails the test through HTTP 500.
	full := catalogPage(append(catalogEntries(1, 98),
		catalogEntry("v99.9.9", true),
		`{"tag_name":"not-a-version","draft":false,"assets":[]}`)...)
	short := catalogPage(catalogEntries(101, 3)...)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("page") {
		case "1":
			io.WriteString(w, full)
		case "2":
			io.WriteString(w, short)
		default:
			http.Error(w, "unexpected page", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	t.Setenv(ReleasesEnv, srv.URL)

	rs, err := FetchReleases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 101 {
		t.Fatalf("got %d releases, want 101 (98 + 3, drafts and malformed skipped)", len(rs))
	}
	if rs[0].Version != "103.0.0" || rs[len(rs)-1].Version != "1.0.0" {
		t.Fatalf("not sorted newest first: %s ... %s", rs[0].Version, rs[len(rs)-1].Version)
	}
	for _, r := range rs {
		if r.Version == "99.9.9" {
			t.Fatal("draft leaked into the catalog")
		}
	}
}

func TestFetchReleasesStopsAtPageBound(t *testing.T) {
	t.Setenv("JUREUMA_USER_HOME", t.TempDir())
	if MaxCatalogPages < 50 {
		t.Fatalf("MaxCatalogPages = %d; the safety bound must cover the installers' 50 pages", MaxCatalogPages)
	}
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		io.WriteString(w, catalogPage(catalogEntries(1, catalogPerPage)...))
	}))
	defer srv.Close()
	t.Setenv(ReleasesEnv, srv.URL)

	rs, err := FetchReleases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := requests.Load(); got != MaxCatalogPages {
		t.Fatalf("requested %d pages, want %d", got, MaxCatalogPages)
	}
	if len(rs) != MaxCatalogPages*catalogPerPage {
		t.Fatalf("got %d releases, want %d", len(rs), MaxCatalogPages*catalogPerPage)
	}
}
