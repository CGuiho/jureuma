package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The tests here use only httptest servers and dummy values; nothing touches
// the real user home or the network.

const dummySHA = "0000000000000000000000000000000000000000000000000000000000000000"

func TestDownload(t *testing.T) {
	t.Setenv("ZEZE_USER_HOME", t.TempDir())
	const payload = "dummy asset bytes"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1.2.3/asset.bin" {
			io.WriteString(w, payload)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()
	t.Setenv(DownloadEnv, srv.URL)
	dir := t.TempDir()
	ctx := context.Background()

	t.Run("success writes 0600 file", func(t *testing.T) {
		dst := filepath.Join(dir, "ok.bin")
		if err := Download(ctx, "v1.2.3", "asset.bin", dst); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(dst)
		if err != nil || string(b) != payload {
			t.Fatalf("contents %q err %v", b, err)
		}
		st, err := os.Stat(dst)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != 0o600 {
			t.Fatalf("mode %v, want 0600", st.Mode())
		}
	})

	t.Run("refuses unsafe names before any request", func(t *testing.T) {
		for _, c := range []struct{ tag, asset string }{
			{"", "asset.bin"},
			{"v1.2.3/../x", "asset.bin"},
			{"v1.2.3 x", "asset.bin"},
			{"v1.2.3", ""},
			{"v1.2.3", ".."},
			{"v1.2.3", "../asset.bin"},
			{"v1.2.3", "dir/asset.bin"},
			{"v1.2.3", "dir\\asset.bin"},
		} {
			dst := filepath.Join(dir, "unsafe.bin")
			if err := Download(ctx, c.tag, c.asset, dst); err == nil {
				t.Errorf("Download(%q, %q) succeeded, want error", c.tag, c.asset)
			}
			if _, err := os.Stat(dst); err == nil {
				t.Errorf("Download(%q, %q) created the destination", c.tag, c.asset)
			}
		}
	})

	t.Run("http error", func(t *testing.T) {
		if err := Download(ctx, "v1.2.3", "missing.bin", filepath.Join(dir, "missing.bin")); err == nil {
			t.Fatal("HTTP 404 must be an error")
		}
	})

	t.Run("existing destination refused", func(t *testing.T) {
		dst := filepath.Join(dir, "exists.bin")
		if err := os.WriteFile(dst, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := Download(ctx, "v1.2.3", "asset.bin", dst); err == nil {
			t.Fatal("overwriting an existing file must be refused")
		}
	})
}

func TestParseChecksums(t *testing.T) {
	sum := strings.Repeat("a", 64)
	tests := []struct {
		name    string
		in      string
		want    map[string]string
		wantErr bool
	}{
		{"valid", sum + "  asset.bin\n", map[string]string{"asset.bin": sum}, false},
		{"binary marker", sum + " *asset.bin\n", map[string]string{"asset.bin": sum}, false},
		{"crlf and several", sum + "  a.bin\r\n" + sum + " *b.bin\r\n", map[string]string{"a.bin": sum, "b.bin": sum}, false},
		{"uppercase hex lowered", strings.Repeat("A", 64) + "  a.bin\n", map[string]string{"a.bin": sum}, false},
		{"empty", "", nil, true},
		{"blank lines only", "\n  \n", nil, true},
		{"one field", "a.bin\n", nil, true},
		{"three fields", sum + "  a.bin extra\n", nil, true},
		{"short hash", strings.Repeat("a", 63) + "  a.bin\n", nil, true},
		{"long hash", strings.Repeat("a", 66) + "  a.bin\n", nil, true},
		{"non-hex hash", strings.Repeat("z", 64) + "  a.bin\n", nil, true},
		{"unsafe name", sum + "  ../a.bin\n", nil, true},
		{"path name", sum + "  dir/a.bin\n", nil, true},
		{"star only name", sum + "  *\n", nil, true},
		{"duplicate entries", sum + "  a.bin\n" + sum + "  *a.bin\n", nil, true},
	}
	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			got, err := ParseChecksums([]byte(c.in))
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParseChecksums(%q) succeeded, want error", c.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseChecksums(%q): %v", c.in, err)
			}
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for k, v := range c.want {
				if got[k] != v {
					t.Fatalf("got %v, want %v", got, c.want)
				}
			}
		})
	}
}

// artJSON builds one artifacts.json entry with a dummy checksum.
func artJSON(id, kind, asset, installedPath string) string {
	return fmt.Sprintf(`{"id":%q,"kind":%q,"asset":%q,"os":"any","arch":"any","sha256":%q,"installedPath":%q,"projections":[]}`,
		id, kind, asset, dummySHA, installedPath)
}

// artJSONPlat builds one artifacts.json entry for a specific target.
func artJSONPlat(id, kind, asset, os, arch, installedPath string) string {
	return fmt.Sprintf(`{"id":%q,"kind":%q,"asset":%q,"os":%q,"arch":%q,"sha256":%q,"installedPath":%q,"projections":[]}`,
		id, kind, asset, os, arch, dummySHA, installedPath)
}

// manJSON builds one artifacts.json document.
func manJSON(name, version string, arts ...string) string {
	return fmt.Sprintf(`{"schema":1,"name":%q,"version":%q,"tag":"v%s","artifacts":[%s]}`,
		name, version, version, strings.Join(arts, ","))
}

func TestParseManifest(t *testing.T) {
	valid := manJSON("zeze", "1.2.3",
		artJSON("payload-linux-amd64", "binary", "zeze-linux-amd64", "versions/1.2.3/zeze"),
		artJSON("skill-guiho-s-zeze", "skill", "guiho-s-zeze.zip", "versions/1.2.3/artifacts/skills/guiho-s-zeze"),
	)
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"valid", valid, false},
		{"valid without artifacts", manJSON("zeze", "1.2.3"), false},
		{"malformed json", `{"schema":1`, true},
		{"not an object", `[]`, true},
		// cli mismatch
		{"wrong cli name", manJSON("other", "1.2.3"), true},
		{"empty cli name", manJSON("", "1.2.3"), true},
		// version mismatches
		{"invalid version", manJSON("zeze", "1.2"), true},
		{"empty version", manJSON("zeze", ""), true},
		{"version prefix does not match path", manJSON("zeze", "1.2.3", artJSON("a", "binary", "a.bin", "versions/1.2.2/a.bin")), true},
		// unsafe names and paths
		{"unsafe asset with slash", manJSON("zeze", "1.2.3", artJSON("a", "binary", "dir/a.bin", "versions/1.2.3/a.bin")), true},
		{"empty asset", manJSON("zeze", "1.2.3", artJSON("a", "binary", "", "versions/1.2.3/a.bin")), true},
		{"dotdot asset", manJSON("zeze", "1.2.3", artJSON("a", "binary", "..", "versions/1.2.3/a.bin")), true},
		{"empty installed path", manJSON("zeze", "1.2.3", artJSON("a", "binary", "a.bin", "")), true},
		{"prefix only installed path", manJSON("zeze", "1.2.3", artJSON("a", "binary", "a.bin", "versions/1.2.3")), true},
		{"trailing slash installed path", manJSON("zeze", "1.2.3", artJSON("a", "binary", "a.bin", "versions/1.2.3/")), true},
		{"dotdot installed path", manJSON("zeze", "1.2.3", artJSON("a", "binary", "a.bin", "versions/1.2.3/../evil")), true},
		{"dot prefix installed path", manJSON("zeze", "1.2.3", artJSON("a", "binary", "a.bin", "./versions/1.2.3/a.bin")), true},
		{"doubled slash installed path", manJSON("zeze", "1.2.3", artJSON("a", "binary", "a.bin", "versions//1.2.3/a.bin")), true},
		{"absolute installed path", manJSON("zeze", "1.2.3", artJSON("a", "binary", "a.bin", "/versions/1.2.3/a.bin")), true},
		{"backslash installed path", manJSON("zeze", "1.2.3", artJSON("a", "binary", "a.bin", `versions/1.2.3/a\b`)), true},
		{"other version installed path", manJSON("zeze", "1.2.3", artJSON("a", "binary", "a.bin", "versions/other/a.bin")), true},
		// The parent's review closed the W2-reported gap: release.go now
		// validates the manifest protocol ("schema") version and rejects
		// artifacts that could be installed together onto the same path.
		// Only artifacts.json format version 1 is understood.
		{"schema version 2 rejected", strings.Replace(valid, `"schema":1`, `"schema":2`, 1), true},
		{"schema version 0 rejected", strings.Replace(valid, `"schema":1`, `"schema":0`, 1), true},
		{"missing schema version rejected", strings.Replace(valid, `"schema":1,`, "", 1), true},
		{"duplicate installed paths rejected", manJSON("zeze", "1.2.3",
			artJSON("a", "binary", "a.bin", "versions/1.2.3/a.bin"),
			artJSON("b", "skill", "b.zip", "versions/1.2.3/a.bin")), true},
		{"platform target overlapping an any target rejected", manJSON("zeze", "1.2.3",
			artJSONPlat("a", "binary", "zeze-linux-amd64", "linux", "amd64", "versions/1.2.3/a.bin"),
			artJSON("b", "prompt", "b.md", "versions/1.2.3/a.bin")), true},
		{"same installed path across distinct platforms allowed", manJSON("zeze", "1.2.3",
			artJSONPlat("a", "binary", "zeze-linux-amd64", "linux", "amd64", "versions/1.2.3/zeze"),
			artJSONPlat("b", "binary", "zeze-linux-arm64", "linux", "arm64", "versions/1.2.3/zeze"),
			artJSONPlat("c", "launcher", "zeze-launcher-linux-amd64", "linux", "amd64", "$HOME/.guiho/bin/zeze"),
			artJSONPlat("d", "launcher", "zeze-launcher-linux-arm64", "linux", "arm64", "$HOME/.guiho/bin/zeze")), false},
	}
	for _, c := range tests {
		t.Run(c.name, func(t *testing.T) {
			m, err := ParseManifest([]byte(c.in))
			if c.wantErr {
				if err == nil {
					t.Fatalf("ParseManifest(%s) succeeded, want error", c.in)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseManifest(%s): %v", c.in, err)
			}
			if m.Name != "zeze" || m.Version == "" {
				t.Fatalf("unexpected manifest %+v", m)
			}
		})
	}
}

func TestForPlatform(t *testing.T) {
	m := &Manifest{Version: "1.2.3", Artifacts: []Artifact{
		{Kind: "binary", Asset: PayloadAsset("linux", "amd64"), OS: "linux", Arch: "amd64"},
		{Kind: "binary", Asset: PayloadAsset("linux", "arm64"), OS: "linux", Arch: "arm64"},
		{Kind: "binary", Asset: PayloadAsset("windows", "amd64"), OS: "windows", Arch: "amd64"},
		{Kind: "skill", Asset: "guiho-s-zeze.zip", OS: "any", Arch: "any"},
		{Kind: "prompt", Asset: "guiho-p-zeze.md", OS: "any", Arch: "any"},
	}}
	// Matching is by asset name (PayloadAsset); the os/arch fields are
	// informational and every non-binary artifact is always included.
	tests := []struct {
		goos, goarch string
		want         []string
	}{
		{"linux", "amd64", []string{"zeze-linux-amd64", "guiho-s-zeze.zip", "guiho-p-zeze.md"}},
		{"windows", "amd64", []string{"zeze-windows-amd64.exe", "guiho-s-zeze.zip", "guiho-p-zeze.md"}},
		{"darwin", "arm64", []string{"guiho-s-zeze.zip", "guiho-p-zeze.md"}},
	}
	for _, c := range tests {
		t.Run(c.goos+"/"+c.goarch, func(t *testing.T) {
			got := m.ForPlatform(c.goos, c.goarch)
			if len(got) != len(c.want) {
				t.Fatalf("got %v, want %v", got, c.want)
			}
			for i := range got {
				if got[i].Asset != c.want[i] {
					t.Fatalf("got %v, want %v", got, c.want)
				}
			}
		})
	}
}
