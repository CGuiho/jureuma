package update

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

// MaxAssetSize bounds every downloaded release asset.
const MaxAssetSize = 256 << 20

// DownloadTimeout bounds one asset download.
const DownloadTimeout = 5 * time.Minute

// AssetURL returns the download URL of asset in the release tagged tag.
func AssetURL(tag, asset string) string {
	base := DownloadBase
	if u := os.Getenv(DownloadEnv); u != "" {
		base = u
	}
	return strings.TrimRight(base, "/") + "/" + tag + "/" + asset
}

// ValidAssetName reports whether name is a plain file name.
func ValidAssetName(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, `/\:`) && !strings.ContainsRune(name, 0)
}

// ValidTag reports whether tag only uses [0-9A-Za-z.+-].
func ValidTag(tag string) bool {
	if tag == "" {
		return false
	}
	for _, r := range tag {
		if !(r == '.' || r == '+' || r == '-' || (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
			return false
		}
	}
	return true
}

// Download fetches asset of the release tagged tag into dst (created with
// mode 0600). It refuses invalid names and assets above MaxAssetSize.
func Download(ctx context.Context, tag, asset, dst string) error {
	if !ValidTag(tag) || !ValidAssetName(asset) {
		return fmt.Errorf("unsafe release tag or asset name")
	}
	ctx, cancel := context.WithTimeout(ctx, DownloadTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, AssetURL(tag, asset), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "zeze")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("download of %s failed", asset)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download of %s returned HTTP %d", asset, resp.StatusCode)
	}
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, io.LimitReader(resp.Body, MaxAssetSize+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("download of %s failed", asset)
	}
	if n > MaxAssetSize {
		return fmt.Errorf("%s is larger than %d MiB", asset, MaxAssetSize>>20)
	}
	return nil
}

// ParseChecksums parses a checksums.txt ("<sha256>  <name>" per line, an
// optional "*" before the name). Empty files, malformed lines and duplicate
// names are errors.
func ParseChecksums(data []byte) (map[string]string, error) {
	out := map[string]string{}
	for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 2 {
			return nil, errors.New("checksums.txt is malformed")
		}
		sum, name := strings.ToLower(f[0]), strings.TrimPrefix(f[1], "*")
		if b, err := hex.DecodeString(sum); err != nil || len(b) != 32 || !ValidAssetName(name) {
			return nil, errors.New("checksums.txt is malformed")
		}
		if _, dup := out[name]; dup {
			return nil, errors.New("checksums.txt has duplicate entries")
		}
		out[name] = sum
	}
	if len(out) == 0 {
		return nil, errors.New("checksums.txt is empty")
	}
	return out, nil
}

// Manifest is a release's artifacts.json.
type Manifest struct {
	Schema    int        `json:"schema"`
	Name      string     `json:"name"`
	Version   string     `json:"version"`
	Tag       string     `json:"tag"`
	Artifacts []Artifact `json:"artifacts"`
}

// Artifact is one entry of Manifest.Artifacts. InstalledPath is relative to
// ~/.guiho/zeze and, except for kind "launcher", always below
// versions/<version>/. Launcher artifacts install to the shared
// ~/.guiho/bin directory and carry that absolute-convention path instead.
type Artifact struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	Asset         string   `json:"asset"`
	OS            string   `json:"os"`
	Arch          string   `json:"arch"`
	SHA256        string   `json:"sha256"`
	InstalledPath string   `json:"installedPath"`
	Projections   []string `json:"projections"`
}

// launcherPaths are the only installed paths a kind "launcher" artifact may
// declare: the stable launcher in the shared ~/.guiho/bin directory.
var launcherPaths = map[string]bool{
	"$HOME/.guiho/bin/zeze":     true,
	"$HOME/.guiho/bin/zeze.exe": true,
}

// ParseManifest parses and checks artifacts.json: the document must be format
// version 1, describe zeze and carry a valid version; every asset name is
// plain; every non-launcher installed path is a clean relative path below
// versions/<version>/; every launcher installed path is the shared command
// path for its platform; and no two artifacts that can be installed together
// (the same path with overlapping platforms) are allowed.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, errors.New("artifacts.json is malformed")
	}
	if m.Name != "zeze" {
		return nil, errors.New("artifacts.json does not describe zeze")
	}
	// Only artifacts.json format version 1 is understood; a document from a
	// different protocol version must be refused rather than misread.
	if m.Schema != 1 {
		return nil, errors.New("artifacts.json has an unsupported schema version")
	}
	if _, ok := Parse(m.Version); !ok {
		return nil, errors.New("artifacts.json has an invalid version")
	}
	prefix := "versions/" + m.Version + "/"
	type target struct{ path, os, arch string }
	var seen []target
	for _, a := range m.Artifacts {
		if !ValidAssetName(a.Asset) {
			return nil, errors.New("artifacts.json names an unsafe asset")
		}
		p := a.InstalledPath
		if a.Kind == "launcher" {
			if !launcherPaths[p] {
				return nil, errors.New("artifacts.json has an unsafe launcher installed path")
			}
		} else if strings.Contains(p, `\`) || path.Clean(p) != p || !strings.HasPrefix(p, prefix) || len(p) == len(prefix) {
			return nil, errors.New("artifacts.json has an unsafe installed path")
		}
		for _, s := range seen {
			if s.path == p && overlapsTarget(s.os, a.OS) && overlapsTarget(s.arch, a.Arch) {
				return nil, errors.New("artifacts.json has duplicate installed paths")
			}
		}
		seen = append(seen, target{p, a.OS, a.Arch})
	}
	return &m, nil
}

// overlapsTarget reports whether two artifact targets can be installed
// together: "any" matches every platform.
func overlapsTarget(a, b string) bool {
	return a == b || a == "any" || b == "any"
}

// ForPlatform returns the artifacts that belong to goos/goarch: that
// platform's payload and stable launcher and every platform-independent
// artifact. Launcher artifacts are downloaded and verified with the release
// but install to the shared ~/.guiho/bin directory, never into a version
// directory.
func (m *Manifest) ForPlatform(goos, goarch string) []Artifact {
	var out []Artifact
	for _, a := range m.Artifacts {
		switch a.Kind {
		case "binary":
			if a.Asset == PayloadAsset(goos, goarch) {
				out = append(out, a)
			}
		case "launcher":
			if a.Asset == LauncherAsset(goos, goarch) {
				out = append(out, a)
			}
		default:
			out = append(out, a)
		}
	}
	return out
}
