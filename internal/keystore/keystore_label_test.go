package keystore

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/CGuiho/zeze/internal/crypt"
)

// entryFor returns the listed entry for keyID.
func entryFor(t *testing.T, s *Store, keyID string) Entry {
	t.Helper()
	entries, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, en := range entries {
		if en.KeyID == keyID {
			return en
		}
	}
	t.Fatalf("key %s not listed", keyID)
	return Entry{}
}

func TestSaveLabeledRoundTrip(t *testing.T) {
	sk, id := newKey(t)
	s := newStore(t, nil)
	path, err := s.SaveLabeled(sk, "zeze")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if raw["label"] != "zeze" {
		t.Fatalf("key file label = %v, want zeze", raw["label"])
	}
	// The key still loads through the normal lookup path.
	got, src, err := s.Find(id)
	if err != nil {
		t.Fatal(err)
	}
	if !crypt.SameKey(got, sk) {
		t.Fatalf("Find returned a different key (source: %s)", src)
	}
	// And the listing surfaces the label and the public key.
	en := entryFor(t, s, id)
	if en.Label != "zeze" {
		t.Fatalf("listed label = %q, want zeze", en.Label)
	}
	if en.PublicKey != sk.Public().String() {
		t.Fatal("listed public key does not match")
	}
	if en.Retired {
		t.Fatal("freshly saved key listed as retired")
	}
	// No listing surface may carry private material.
	blob, err := json.Marshal(en)
	if err != nil {
		t.Fatal(err)
	}
	secret := strings.TrimPrefix(sk.String(), crypt.PrivateKeyPrefix)
	if strings.Contains(string(blob), secret) || strings.Contains(string(blob), secret[:10]) {
		t.Fatal("listed entry leaks private key material")
	}
}

func TestLegacyKeyFileWithoutLabel(t *testing.T) {
	sk, id := newKey(t)
	s := newStore(t, nil)
	path, err := s.Save(sk)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "label") {
		t.Fatal("an unlabeled save must not write a label field")
	}
	if _, _, err := s.Find(id); err != nil {
		t.Fatalf("legacy key file no longer loads: %v", err)
	}
	en := entryFor(t, s, id)
	if en.Label != "" {
		t.Fatalf("legacy key listed with label %q, want empty", en.Label)
	}
}

func TestSaveLabeledUpdatesLabelKeepsCreated(t *testing.T) {
	sk, id := newKey(t)
	s := newStore(t, nil)
	path, err := s.SaveLabeled(sk, "first")
	if err != nil {
		t.Fatal(err)
	}
	first, ok := readKeyMeta(path, id)
	if !ok {
		t.Fatal("cannot read key metadata")
	}
	if first.Label != "first" {
		t.Fatalf("stored label = %q, want first", first.Label)
	}
	if _, err := s.SaveLabeled(sk, "second"); err != nil {
		t.Fatal(err)
	}
	second, ok := readKeyMeta(path, id)
	if !ok {
		t.Fatal("cannot read key metadata after relabel")
	}
	if second.Label != "second" {
		t.Fatalf("stored label = %q, want second", second.Label)
	}
	if second.Created != first.Created {
		t.Fatalf("relabel changed created: %q -> %q", first.Created, second.Created)
	}
	if got := entryFor(t, s, id).Label; got != "second" {
		t.Fatalf("listed label = %q, want second", got)
	}
}

func TestListSurfacesLabels(t *testing.T) {
	s := newStore(t, nil)
	labeled, labeledID := newKey(t)
	legacy, legacyID := newKey(t)
	retired, retiredID := newKey(t)
	if _, err := s.SaveLabeled(labeled, "labeled-project"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Save(legacy); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SaveLabeled(retired, "retired-project"); err != nil {
		t.Fatal(err)
	}
	if err := s.Retire(retiredID); err != nil {
		t.Fatal(err)
	}
	entries, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("listed %d keys, want 3", len(entries))
	}
	// Active keys come first, each group sorted by key ID.
	if entries[0].Retired || entries[1].Retired || !entries[2].Retired {
		t.Fatalf("order/state wrong: %+v", entries)
	}
	if entries[2].KeyID != retiredID {
		t.Fatalf("retired key listed as %s", entries[2].KeyID)
	}
	if got := entryFor(t, s, labeledID).Label; got != "labeled-project" {
		t.Fatalf("labeled key label = %q", got)
	}
	if got := entryFor(t, s, legacyID).Label; got != "" {
		t.Fatalf("legacy key label = %q, want empty", got)
	}
	if got := entryFor(t, s, retiredID).Label; got != "retired-project" {
		t.Fatalf("retired key label = %q", got)
	}
	for _, en := range entries {
		if en.PublicKey == "" {
			t.Fatalf("key %s listed without a public key", en.KeyID)
		}
	}
}
