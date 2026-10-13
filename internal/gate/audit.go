package gate

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// AuditFileName is the audit log file name under the CLI home.
const AuditFileName = "audit.log"

// AuditEvent is one audit log line. It never contains configuration values,
// plaintext, or key material: only the command, file paths, the outcome, and
// the names of agent markers.
type AuditEvent struct {
	Time    time.Time
	Command string
	Paths   []string
	Outcome string
	Markers []string
	Bypass  bool
}

type auditLine struct {
	Time    string   `json:"time"`
	Command string   `json:"command"`
	Paths   []string `json:"paths"`
	Outcome string   `json:"outcome"`
	Markers []string `json:"markers"`
	Bypass  bool     `json:"bypass"`
}

// ErrAuditUnsafe is returned when the audit log path is not a regular file.
var ErrAuditUnsafe = errors.New("audit log is not a regular file")

// Audit appends one JSON line to <home>/audit.log (0600). Never values.
func Audit(home string, ev AuditEvent) error {
	if home == "" {
		return errors.New("audit: empty home directory")
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		return err
	}
	t := ev.Time
	if t.IsZero() {
		t = time.Now()
	}
	line := auditLine{
		Time:    t.UTC().Format(time.RFC3339Nano),
		Command: ev.Command,
		Paths:   ev.Paths,
		Outcome: ev.Outcome,
		Markers: ev.Markers,
		Bypass:  ev.Bypass,
	}
	if line.Paths == nil {
		line.Paths = []string{}
	}
	if line.Markers == nil {
		line.Markers = []string{}
	}
	data, err := json.Marshal(line)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	path := filepath.Join(home, AuditFileName)
	if fi, err := os.Lstat(path); err == nil {
		if !fi.Mode().IsRegular() {
			return ErrAuditUnsafe
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	if fi, err := f.Stat(); err == nil && fi.Mode().Perm() != 0o600 {
		_ = f.Chmod(0o600)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
