package config

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProtectionsSealAndVerify(t *testing.T) {
	home := isolate(t)
	g := DefaultGlobal()
	g.Protections.AgentEnvMarkers = []string{"MY_AGENT"}
	if err := SealProtections(home, g); err != nil {
		t.Fatal(err)
	}
	key, err := os.ReadFile(filepath.Join(home, "state", ProtectionsKeyFile))
	if err != nil || len(key) != 32 {
		t.Fatalf("key: %d %v", len(key), err)
	}
	if runtime.GOOS != "windows" {
		st, _ := os.Stat(filepath.Join(home, "state", ProtectionsKeyFile))
		if st.Mode().Perm() != 0o600 {
			t.Errorf("key mode %v", st.Mode().Perm())
		}
	}
	digest, _ := os.ReadFile(filepath.Join(home, "state", ProtectionsDigestFile))
	if bytes.Contains(digest, key) || strings.Contains(string(digest), "MY_AGENT") {
		t.Error("digest file leaks key or content")
	}

	cases := []struct {
		name string
		mod  func(*Global)
		want bool
	}{
		{"unchanged", func(*Global) {}, true},
		{"nil allow equals true", func(g *Global) { g.Protections.AllowBypass = nil }, true},
		{"agent block ignored", func(g *Global) { g.Agent.Evolution.Upgrade = "disabled" }, true},
		{"allow flipped", func(g *Global) { f := false; g.Protections.AllowBypass = &f }, false},
		{"marker removed", func(g *Global) { g.Protections.AgentEnvMarkers = nil }, false},
		{"marker added", func(g *Global) { g.Protections.AgentEnvMarkers = append(g.Protections.AgentEnvMarkers, "B") }, false},
		{"marker renamed", func(g *Global) { g.Protections.AgentEnvMarkers = []string{"OTHER"} }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := *g
			c.Protections.AgentEnvMarkers = append([]string(nil), g.Protections.AgentEnvMarkers...)
			tc.mod(&c)
			ok, err := ProtectionsIntact(home, &c)
			if err != nil || ok != tc.want {
				t.Errorf("got %v %v want %v", ok, err, tc.want)
			}
		})
	}

	// Re-sealing reuses the same key.
	if err := SealProtections(home, g); err != nil {
		t.Fatal(err)
	}
	key2, _ := os.ReadFile(filepath.Join(home, "state", ProtectionsKeyFile))
	if !bytes.Equal(key, key2) {
		t.Error("key regenerated")
	}
}

func TestProtectionsUnsealed(t *testing.T) {
	home := isolate(t)
	for _, tc := range []struct {
		g    *Global
		want bool
	}{
		{nil, true},
		{&Global{}, true},
		{DefaultGlobal(), true},
		{&Global{Protections: Protections{AllowBypass: boolp(false)}}, false},
		{&Global{Protections: Protections{AgentEnvMarkers: []string{"A"}}}, false},
	} {
		ok, err := ProtectionsIntact(home, tc.g)
		if err != nil || ok != tc.want {
			t.Errorf("%+v: %v %v", tc.g, ok, err)
		}
	}
}

func TestProtectionsTampered(t *testing.T) {
	home := isolate(t)
	g := DefaultGlobal()
	if err := SealProtections(home, g); err != nil {
		t.Fatal(err)
	}
	dp := filepath.Join(home, "state", ProtectionsDigestFile)
	for _, content := range []string{"not json", `{"version":2,"algorithm":"HMAC-SHA256","digest":"00"}`, `{"version":1,"algorithm":"HMAC-SHA256","digest":"zz"}`, `{"version":1,"algorithm":"HMAC-SHA256","digest":"00"}`} {
		os.WriteFile(dp, []byte(content), 0o600)
		if ok, err := ProtectionsIntact(home, g); ok || err != nil {
			t.Errorf("%q: %v %v", content, ok, err)
		}
	}
	// A different key invalidates the seal.
	if err := SealProtections(home, g); err != nil {
		t.Fatal(err)
	}
	kp := filepath.Join(home, "state", ProtectionsKeyFile)
	os.WriteFile(kp, bytes.Repeat([]byte{7}, 32), 0o600)
	if ok, _ := ProtectionsIntact(home, g); ok {
		t.Error("intact with another key")
	}
	// A corrupt key is an error that does not include key bytes.
	os.WriteFile(kp, []byte("short"), 0o600)
	_, err := ProtectionsIntact(home, g)
	if err == nil || strings.Contains(err.Error(), "short") {
		t.Errorf("corrupt key: %v", err)
	}
	if err := SealProtections(home, g); err == nil {
		t.Error("seal with corrupt key succeeded")
	}
}
