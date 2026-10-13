//go:build !windows

package keystore

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInsecureKeyFiles(t *testing.T) {
	sk, id := newKey(t)

	t.Run("group readable", func(t *testing.T) {
		s := newStore(t, nil)
		p, _ := s.Save(sk)
		os.Chmod(p, 0o640)
		_, _, err := s.Find(id)
		var ie *InvalidKeyError
		if !errors.As(err, &ie) || !strings.Contains(ie.Reason, "0600") {
			t.Fatalf("want permission error, got %v", err)
		}
		assertNoSecret(t, err, sk)
	})

	t.Run("read-only owner ok", func(t *testing.T) {
		s := newStore(t, nil)
		p, _ := s.Save(sk)
		os.Chmod(p, 0o400)
		if _, _, err := s.Find(id); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("symlink", func(t *testing.T) {
		s := newStore(t, nil)
		p, _ := s.Save(sk)
		real := filepath.Join(filepath.Dir(s.Home), "real.json")
		os.Rename(p, real)
		if err := os.Symlink(real, p); err != nil {
			t.Fatal(err)
		}
		_, _, err := s.Find(id)
		var ie *InvalidKeyError
		if !errors.As(err, &ie) || !strings.Contains(ie.Reason, "symbolic link") {
			t.Fatalf("want symlink error, got %v", err)
		}
		if err := s.Retire(id); err == nil {
			t.Fatal("retired a symlink")
		}
	})

	t.Run("directory", func(t *testing.T) {
		s := newStore(t, nil)
		os.MkdirAll(filepath.Join(s.Home, "keys", id+".json"), 0o700)
		if _, _, err := s.Find(id); !errors.As(err, new(*InvalidKeyError)) {
			t.Fatalf("want InvalidKeyError, got %v", err)
		}
	})
}
