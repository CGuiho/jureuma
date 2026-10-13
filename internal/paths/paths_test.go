package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTempNameKeepsMarker(t *testing.T) {
	dir := t.TempDir()
	for _, base := range []string{"config.unencrypted.json", "apps.api.unencrypted.jsonc", "config.encrypted.json"} {
		pat := tempPattern(base)
		f, err := os.CreateTemp(dir, pat)
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(f.Name())
		f.Close()
		ext := filepath.Ext(base)
		ok, _ := filepath.Match("*.unencrypted"+ext, name)
		if base == "config.encrypted.json" {
			ok, _ = filepath.Match("*.encrypted.json", name)
		}
		if !ok || !IsStaleTemp(name) {
			t.Errorf("%s: temp name %s does not keep the marker and extension", base, name)
		}
	}
	if got := tempPattern("cache.json"); got != ".cache.json.tmp-*" {
		t.Errorf("plain name pattern = %s", got)
	}
}

func TestAtomicWriteLeavesNoTemp(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.unencrypted.json")
	if err := AtomicWrite(p, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	ents, _ := os.ReadDir(dir)
	if len(ents) != 1 {
		t.Fatalf("left %d entries", len(ents))
	}
}

func TestHomeRelocated(t *testing.T) {
	h := t.TempDir()
	t.Setenv("HOME", h)
	t.Setenv("USERPROFILE", h)
	t.Setenv(HomeOverrideEnv, "")
	if HomeRelocated() {
		t.Fatal("no override is not relocated")
	}
	t.Setenv(HomeOverrideEnv, h)
	if HomeRelocated() {
		t.Fatal("override equal to HOME is not relocated")
	}
	t.Setenv(HomeOverrideEnv, t.TempDir())
	if !HomeRelocated() {
		t.Fatal("override elsewhere is relocated")
	}
}
