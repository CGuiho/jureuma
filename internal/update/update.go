// Package update implements the release catalog lookup, SemVer comparison,
// the cached single-flight startup update check, and the reinstall commands
// printed by "zeze upgrade".
package update

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/CGuiho/zeze/internal/paths"
)

// Repository coordinates.
const (
	Repo         = "CGuiho/zeze"
	RepoURL      = "https://github.com/" + Repo
	InstallSh    = "https://raw.githubusercontent.com/" + Repo + "/main/devops/install.sh"
	InstallPs1   = "https://raw.githubusercontent.com/" + Repo + "/main/devops/install.ps1"
	ReleasesAPI  = "https://api.github.com/repos/" + Repo + "/releases"
	ReleasesEnv  = "ZEZE_RELEASES_URL" // test/sandbox override of ReleasesAPI
	DownloadBase = "https://github.com/" + Repo + "/releases/download"
	DownloadEnv  = "ZEZE_DOWNLOAD_BASE_URL" // test/sandbox override of DownloadBase
	WorkerEnv    = "ZEZE_UPDATE_WORKER"
	DisableEnv   = "ZEZE_NO_UPDATE_CHECK"
	CacheFile    = "cache.json"
	LeaseFile    = "update-worker.lease"
	CacheTTL     = 24 * time.Hour
	MaxLeaseAge  = 2 * time.Minute
	FetchTimeout = 15 * time.Second
)

// Semver is a parsed SemVer 2.0.0 version.
type Semver struct {
	Major, Minor, Patch int
	Pre                 []string
	Raw                 string
}

// Parse parses "1.2.3", "1.2.3-beta.1+build" or the same with a "v" prefix.
func Parse(s string) (Semver, bool) {
	raw := strings.TrimPrefix(s, "v")
	v := Semver{Raw: raw}
	core := raw
	if i := strings.IndexByte(core, '+'); i >= 0 {
		core = core[:i]
	}
	if i := strings.IndexByte(core, '-'); i >= 0 {
		pre := core[i+1:]
		core = core[:i]
		if pre == "" {
			return v, false
		}
		v.Pre = strings.Split(pre, ".")
		for _, p := range v.Pre {
			if p == "" {
				return v, false
			}
		}
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return v, false
	}
	nums := make([]int, 3)
	for i, p := range parts {
		if p == "" || (len(p) > 1 && p[0] == '0') {
			return v, false
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		nums[i] = n
	}
	v.Major, v.Minor, v.Patch = nums[0], nums[1], nums[2]
	return v, true
}

// Channel returns "stable" or the first prerelease identifier.
func (v Semver) Channel() string {
	if len(v.Pre) == 0 {
		return "stable"
	}
	return v.Pre[0]
}

// Compare returns -1, 0 or 1 following SemVer precedence.
func Compare(a, b Semver) int {
	for _, d := range []int{a.Major - b.Major, a.Minor - b.Minor, a.Patch - b.Patch} {
		if d < 0 {
			return -1
		}
		if d > 0 {
			return 1
		}
	}
	if len(a.Pre) == 0 && len(b.Pre) == 0 {
		return 0
	}
	if len(a.Pre) == 0 {
		return 1
	}
	if len(b.Pre) == 0 {
		return -1
	}
	for i := 0; i < len(a.Pre) && i < len(b.Pre); i++ {
		x, y := a.Pre[i], b.Pre[i]
		xn, xerr := strconv.Atoi(x)
		yn, yerr := strconv.Atoi(y)
		switch {
		case xerr == nil && yerr == nil:
			if xn != yn {
				if xn < yn {
					return -1
				}
				return 1
			}
		case xerr == nil:
			return -1
		case yerr == nil:
			return 1
		default:
			if x != y {
				if x < y {
					return -1
				}
				return 1
			}
		}
	}
	switch {
	case len(a.Pre) < len(b.Pre):
		return -1
	case len(a.Pre) > len(b.Pre):
		return 1
	}
	return 0
}

// Release is one published release.
type Release struct {
	Version string
	Tag     string
	Channel string
	URL     string
	Assets  []string
}

// AgentAssets are the agent artifacts every release must carry (the same
// list as AGENT_ASSETS in install.sh and install.ps1).
var AgentAssets = []string{"guiho-s-zeze.zip", "zeze-instruction.md", "guiho-p-zeze.md", "guiho-p-zeze-install.md", "guiho-p-zeze-uninstall.md"}

// PayloadAsset returns the release asset name of the payload for a platform.
func PayloadAsset(goos, goarch string) string {
	name := "zeze-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// LauncherAsset returns the release asset name of the stable launcher for a
// platform. The launcher installs to the shared ~/.guiho/bin directory and is
// never part of a versions/<version>/ payload directory.
func LauncherAsset(goos, goarch string) string {
	name := "zeze-launcher-" + goos + "-" + goarch
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// RequiredAssets returns every asset a release must carry to be installable
// on goos/goarch: the payload, the stable launcher, checksums.txt,
// artifacts.json and the agent artifacts.
func RequiredAssets(goos, goarch string) []string {
	return append([]string{PayloadAsset(goos, goarch), LauncherAsset(goos, goarch), "checksums.txt", "artifacts.json"}, AgentAssets...)
}

// Compatible reports whether r carries every asset RequiredAssets lists.
func (r Release) Compatible(goos, goarch string) bool {
	have := map[string]bool{}
	for _, a := range r.Assets {
		have[a] = true
	}
	for _, a := range RequiredAssets(goos, goarch) {
		if !have[a] {
			return false
		}
	}
	return true
}

// Installable returns the releases of rs that are Compatible with the
// running platform, in order.
func Installable(rs []Release) []Release {
	var out []Release
	for _, r := range rs {
		if r.Compatible(runtime.GOOS, runtime.GOARCH) {
			out = append(out, r)
		}
	}
	return out
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	Draft   bool   `json:"draft"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name string `json:"name"`
	} `json:"assets"`
}

// ErrNoReleases is returned when the catalog has no usable release.
var ErrNoReleases = errors.New("no published releases found")

// Catalog pagination: the release API serves catalogPerPage releases per
// page, and FetchReleases walks pages until one returns fewer than
// catalogPerPage items. MaxCatalogPages is the documented safety bound; it is
// no smaller than the 50-page bound of devops/install.sh and devops/install.ps1
// so channel discovery sees at least as many releases as the installers.
const (
	catalogPerPage  = 100
	MaxCatalogPages = 50
)

// FetchReleases reads every page of the release catalog (bounded by
// MaxCatalogPages, stopping at the first page with fewer than catalogPerPage
// items), skipping drafts and malformed versions, sorted newest first.
func FetchReleases(ctx context.Context) ([]Release, error) {
	base := ReleasesAPI
	if u := os.Getenv(ReleasesEnv); u != "" {
		base = u
	}
	ctx, cancel := context.WithTimeout(ctx, FetchTimeout)
	defer cancel()
	client := &http.Client{Timeout: FetchTimeout}
	var out []Release
	for page := 1; page <= MaxCatalogPages; page++ {
		url := fmt.Sprintf("%s?per_page=%d&page=%d", base, catalogPerPage, page)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "zeze")
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("release catalog request failed")
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("release catalog read failed")
		}
		if resp.StatusCode == http.StatusNotFound {
			return nil, ErrNoReleases
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("release catalog returned HTTP %d", resp.StatusCode)
		}
		var rs []ghRelease
		if err := json.Unmarshal(body, &rs); err != nil {
			return nil, fmt.Errorf("release catalog is malformed")
		}
		for _, r := range rs {
			if r.Draft {
				continue
			}
			v, ok := Parse(r.TagName)
			if !ok {
				continue
			}
			rel := Release{Version: v.Raw, Tag: r.TagName, Channel: v.Channel(), URL: r.HTMLURL}
			for _, a := range r.Assets {
				rel.Assets = append(rel.Assets, a.Name)
			}
			out = append(out, rel)
		}
		if len(rs) < catalogPerPage {
			break
		}
	}
	if len(out) == 0 {
		return nil, ErrNoReleases
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, _ := Parse(out[i].Version)
		b, _ := Parse(out[j].Version)
		return Compare(a, b) > 0
	})
	return out, nil
}

// Select picks a release among those installable on this platform (see
// Installable): exact version, else highest in channel (default stable).
// Releases without the payload, checksums.txt, artifacts.json or an agent
// artifact are never selected, exactly like the installers.
func Select(rs []Release, version, channel string) (Release, bool) {
	rs = Installable(rs)
	if version != "" {
		want, ok := Parse(version)
		if !ok {
			return Release{}, false
		}
		for _, r := range rs {
			v, _ := Parse(r.Version)
			if Compare(v, want) == 0 {
				return r, true
			}
		}
		return Release{}, false
	}
	if channel == "" {
		channel = "stable"
	}
	for _, r := range rs {
		if r.Channel == channel {
			return r, true
		}
	}
	return Release{}, false
}

// ReinstallCommand returns the platform's full reinstall command.
func ReinstallCommand(goos, version, channel string) string {
	if goos == "windows" {
		sel := ""
		switch {
		case version != "":
			sel = fmt.Sprintf(" -Version '%s'", version)
		case channel != "":
			sel = fmt.Sprintf(" -Channel '%s'", channel)
		}
		return fmt.Sprintf(`powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "& ([scriptblock]::Create((Invoke-RestMethod '%s')))%s"`, InstallPs1, sel)
	}
	switch {
	case version != "":
		return fmt.Sprintf("curl -fsSL %s | sh -s -- --version %s", InstallSh, version)
	case channel != "":
		return fmt.Sprintf("curl -fsSL %s | sh -s -- --channel %s", InstallSh, channel)
	}
	return fmt.Sprintf("curl -fsSL %s | sh", InstallSh)
}

// Cache is the update cache.
type Cache struct {
	LatestVersion       string    `json:"latestVersion"`
	CurrentVersion      string    `json:"currentVersion"`
	NewVersionAvailable bool      `json:"newVersionAvailable"`
	LastCheck           time.Time `json:"lastCheck"`
	UpgradeCommand      string    `json:"upgradeCommand"`
}

// ReadCache returns the cache when it is valid and fresh.
func ReadCache(home string) (*Cache, bool) {
	data, err := os.ReadFile(filepath.Join(home, CacheFile))
	if err != nil || len(data) > 64<<10 {
		return nil, false
	}
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	var c Cache
	if err := dec.Decode(&c); err != nil {
		return nil, false
	}
	if _, ok := Parse(c.LatestVersion); !ok || c.LastCheck.IsZero() {
		return nil, false
	}
	age := time.Since(c.LastCheck)
	if age < 0 || age > CacheTTL {
		return nil, false
	}
	return &c, true
}

// WriteCache writes the cache atomically.
func WriteCache(home string, c *Cache) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	return paths.AtomicWrite(filepath.Join(home, CacheFile), append(data, '\n'), 0o600)
}

// Notice returns the two-line update notice, or "" when nothing qualifies.
func Notice(home, current string) string {
	c, ok := ReadCache(home)
	if !ok || !c.NewVersionAvailable {
		return ""
	}
	latest, ok1 := Parse(c.LatestVersion)
	cur, ok2 := Parse(current)
	if !ok1 || !ok2 || Compare(latest, cur) <= 0 {
		return ""
	}
	return fmt.Sprintf("⚠ New version available: v%s\n  run zeze upgrade to update\n", latest.Raw)
}

// NeedsCheck reports whether the cache is missing, expired or invalid.
func NeedsCheck(home string) bool {
	_, ok := ReadCache(home)
	return !ok
}

// AcquireLease atomically creates the worker lease. It breaks a lease older
// than MaxLeaseAge. ok is false when another valid lease exists.
func AcquireLease(home string) (token string, ok bool) {
	if err := os.MkdirAll(home, 0o700); err != nil {
		return "", false
	}
	path := filepath.Join(home, LeaseFile)
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", false
	}
	token = hex.EncodeToString(b)
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_, werr := fmt.Fprintf(f, "%d %s\n", os.Getpid(), token)
			f.Close()
			if werr != nil {
				os.Remove(path)
				return "", false
			}
			return token, true
		}
		st, serr := os.Stat(path)
		if serr != nil || time.Since(st.ModTime()) < MaxLeaseAge {
			return "", false
		}
		os.Remove(path)
	}
	return "", false
}

// ReleaseLease removes the lease if it still carries token.
func ReleaseLease(home, token string) {
	path := filepath.Join(home, LeaseFile)
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	f := strings.Fields(string(b))
	if len(f) == 2 && f[1] == token {
		os.Remove(path)
	}
}

// Check fetches the catalog and records the result in the cache.
func Check(ctx context.Context, home, current string) (*Cache, error) {
	rs, err := FetchReleases(ctx)
	if err != nil {
		return nil, err
	}
	r, ok := Select(rs, "", "")
	if !ok {
		return nil, ErrNoReleases
	}
	latest, _ := Parse(r.Version)
	c := &Cache{LatestVersion: r.Version, CurrentVersion: current, LastCheck: time.Now().UTC(), UpgradeCommand: "zeze upgrade"}
	if cur, ok := Parse(current); ok {
		c.NewVersionAvailable = Compare(latest, cur) > 0
	}
	if err := WriteCache(home, c); err != nil {
		return nil, err
	}
	return c, nil
}
