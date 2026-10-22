package cli

// The upgrade transaction journal (convention: The upgrade transaction
// journal must distinguish staging, artifact replacement, activation,
// verification, rollback and completion so a later invocation can repair an
// interrupted transaction before dispatching an ordinary command). It lives
// in <home>/state/upgrade-transaction.json and is also the transaction's
// rollback plan: every field is written before the change it describes
// ("write-ahead"), so an interrupted transaction can always be rolled back
// completely from the journal and the backups staged beside it.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CGuiho/zeze/internal/paths"
	"github.com/CGuiho/zeze/internal/update"
)

// The journal file name under <home>/state/.
const upgradeJournalFile = "upgrade-transaction.json"

// The phases the journal distinguishes. They are write-ahead markers: a phase
// names the work that is about to begin, never work that already finished.
const (
	phaseStaging         = "staging"              // staging and candidate verification
	phaseArtifactReplace = "artifact-replacement" // versions/<v>, skills, manifest
	phaseActivation      = "activation"           // the atomic current.json swap
	phaseVerification    = "verification"         // launcher and payload checks
	phaseRollback        = "rollback"             // rollback in progress
	phaseCompletion      = "completion"           // the transaction is finished
)

// upgradeJournal is the on-disk transaction record.
type upgradeJournal struct {
	Schema    int    `json:"schema"`
	Operation string `json:"operation"`
	Phase     string `json:"phase"`
	Version   string `json:"version"`
	Previous  string `json:"previous"`
	PID       int    `json:"pid"`
	Stage     string `json:"stage"`
	Candidate string `json:"candidate,omitempty"`

	// The rollback plan (write-ahead).
	VersionBackup  string            `json:"versionBackup,omitempty"`
	CreatesVersion bool              `json:"createsVersion,omitempty"`
	SkillWrites    []string          `json:"skillWrites,omitempty"`
	SkillBackups   map[string]string `json:"skillBackups,omitempty"`
	WritesManifest bool              `json:"writesManifest,omitempty"`
	ManifestBackup string            `json:"manifestBackup,omitempty"`
	WritesCurrent  bool              `json:"writesCurrent,omitempty"`
	CurrentBackup  string            `json:"currentBackup,omitempty"`
	AgentsTarget   string            `json:"agentsTarget,omitempty"`
	AgentsHad      bool              `json:"agentsHad,omitempty"`
	AgentsBackup   string            `json:"agentsBackup,omitempty"`
}

func newUpgradeJournal(version, previous, stage, candidate string) *upgradeJournal {
	return &upgradeJournal{
		Schema: 1, Operation: "upgrade", Phase: phaseStaging,
		Version: version, Previous: previous,
		PID: os.Getpid(), Stage: stage, Candidate: candidate,
		SkillBackups: map[string]string{},
	}
}

func journalPath(home string) string {
	return filepath.Join(home, "state", upgradeJournalFile)
}

// save writes the journal atomically.
func (j *upgradeJournal) save(home string) error {
	data, err := json.Marshal(j)
	if err != nil {
		return err
	}
	return paths.AtomicWrite(journalPath(home), append(data, '\n'), 0o600)
}

// loadJournal reads and validates the journal. A missing journal returns
// os.ErrNotExist. A journal that cannot be parsed or that names a path the
// transaction could not have written is never acted on and never deleted: it
// is the only rollback record and a human must look at it.
func loadJournal(home string) (*upgradeJournal, error) {
	data, err := os.ReadFile(journalPath(home))
	if err != nil {
		return nil, err
	}
	if len(data) > 1<<20 {
		return nil, errors.New("the upgrade transaction journal is implausibly large")
	}
	var j upgradeJournal
	if err := json.Unmarshal(data, &j); err != nil {
		return nil, errors.New("the upgrade transaction journal is unparseable")
	}
	if err := validateJournal(home, &j); err != nil {
		return nil, err
	}
	return &j, nil
}

// validateJournal rejects a journal that names anything the transaction could
// not have written, so a tampered or corrupt journal can never become a
// deletion or overwrite vector.
func validateJournal(home string, j *upgradeJournal) error {
	if j.Schema != 1 || j.Operation != "upgrade" {
		return errors.New("the upgrade transaction journal has an unsupported schema")
	}
	switch j.Phase {
	case phaseStaging, phaseArtifactReplace, phaseActivation, phaseVerification, phaseRollback, phaseCompletion:
	default:
		return errors.New("the upgrade transaction journal names an unknown phase")
	}
	if _, ok := update.Parse(j.Version); !ok {
		return errors.New("the upgrade transaction journal names an invalid version")
	}
	if j.Previous != "" {
		if _, ok := update.Parse(j.Previous); !ok {
			return errors.New("the upgrade transaction journal names an invalid previous version")
		}
	}
	tempRoot, err := paths.TempDir()
	if err != nil {
		return err
	}
	if !withinNamedDir(tempRoot, "zeze-upgrade-", j.Stage) {
		return errors.New("the upgrade transaction journal names an unexpected staging directory")
	}
	if j.Candidate != "" && !withinNamedDir(filepath.Join(home, "versions"), ".candidate-", j.Candidate) {
		return errors.New("the upgrade transaction journal names an unexpected candidate directory")
	}
	for _, b := range []string{j.VersionBackup, j.ManifestBackup, j.CurrentBackup, j.AgentsBackup} {
		if b != "" && filepath.Dir(b) != j.Stage {
			return errors.New("the upgrade transaction journal names an unexpected backup")
		}
	}
	if j.AgentsTarget != "" && filepath.Base(j.AgentsTarget) != "AGENTS.md" {
		return errors.New("the upgrade transaction journal names an unexpected instruction target")
	}
	dests, err := skillDests()
	if err != nil {
		return err
	}
	inDests := func(target string) bool {
		for _, d := range dests {
			if filepath.Dir(target) == d && validSkillID(filepath.Base(target)) {
				return true
			}
		}
		return false
	}
	for _, t := range j.SkillWrites {
		if !inDests(t) {
			return errors.New("the upgrade transaction journal names an unexpected skill target")
		}
	}
	for t, b := range j.SkillBackups {
		if !inDests(t) || filepath.Dir(b) != j.Stage {
			return errors.New("the upgrade transaction journal names an unexpected skill backup")
		}
	}
	return nil
}

// withinNamedDir reports whether p is a direct child of dir whose name starts
// with prefix.
func withinNamedDir(dir, prefix, p string) bool {
	if p == "" {
		return false
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return false
	}
	return filepath.Dir(abs) == dir && strings.HasPrefix(filepath.Base(abs), prefix)
}

// journalChanged reports whether the transaction reached a change outside its
// staging directory.
func journalChanged(j *upgradeJournal) bool {
	return j.CreatesVersion || j.VersionBackup != "" || len(j.SkillWrites) > 0 ||
		len(j.SkillBackups) > 0 || j.WritesManifest || j.WritesCurrent || j.AgentsTarget != ""
}

// backupPaths lists every staged backup the journal records.
func (j *upgradeJournal) backupPaths() []string {
	out := make([]string, 0, 4+len(j.SkillBackups))
	for _, b := range []string{j.VersionBackup, j.ManifestBackup, j.CurrentBackup, j.AgentsBackup} {
		if b != "" {
			out = append(out, b)
		}
	}
	for _, b := range j.SkillBackups {
		out = append(out, b)
	}
	return out
}

// restoreFile applies the journal's restore rule to one file: a recorded
// backup that still exists replaces the target and is consumed; a target with
// no recorded backup was created by the transaction and is removed; a
// recorded backup that is gone was never written, so the target is left
// alone.
func restoreFile(target, backup string, mode os.FileMode) error {
	if backup == "" {
		if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	data, err := os.ReadFile(backup)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := paths.AtomicWrite(target, data, mode); err != nil {
		return err
	}
	return os.Remove(backup)
}

// restoreTree applies the same rule to a directory tree (a skill projection
// or a versions/<v> directory).
func restoreTree(target, backup string) error {
	if backup == "" {
		return os.RemoveAll(target)
	}
	if _, err := os.Lstat(backup); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return os.Rename(backup, target)
}

// rollbackJournal restores every change the transaction recorded in its
// journal, newest first. It is idempotent: a partially completed rollback is
// finished by the next call. It returns the staged backups that could not be
// restored (they must be kept) and the failures to report.
func rollbackJournal(home string, j *upgradeJournal) (kept []string, err error) {
	var failures []error
	fail := func(e error) {
		if e != nil {
			failures = append(failures, e)
		}
	}
	if j.AgentsTarget != "" {
		fail(restoreFile(j.AgentsTarget, j.AgentsBackup, 0o644))
	}
	if j.WritesCurrent {
		fail(restoreFile(filepath.Join(home, currentFile), j.CurrentBackup, 0o644))
	}
	if j.WritesManifest {
		fail(restoreFile(filepath.Join(home, installedFile), j.ManifestBackup, 0o644))
	}
	targets := make([]string, 0, len(j.SkillWrites)+len(j.SkillBackups))
	seen := map[string]bool{}
	for _, t := range append(append([]string{}, j.SkillWrites...), sortedKeys(j.SkillBackups)...) {
		if !seen[t] {
			seen[t] = true
			targets = append(targets, t)
		}
	}
	sort.Strings(targets)
	for _, t := range targets {
		fail(restoreTree(t, j.SkillBackups[t]))
	}
	if j.CreatesVersion || j.VersionBackup != "" {
		fail(restoreTree(filepath.Join(home, "versions", j.Version), j.VersionBackup))
	}
	if j.Candidate != "" {
		_ = os.RemoveAll(j.Candidate)
	}
	for _, b := range j.backupPaths() {
		if _, err := os.Lstat(b); err == nil {
			kept = append(kept, b)
		}
	}
	return kept, errors.Join(failures...)
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// repairJournal repairs an interrupted upgrade transaction by completing its
// rollback to a consistent state. It is idempotent and never destructive
// beyond the transaction's own staging: only what the journal proves the
// transaction wrote is undone. A journal whose staging directory (and with it
// the staged backups) is gone cannot be rolled back piece by piece without
// risking deletion of pre-existing files, so the installed state is kept.
func repairJournal(home string, out io.Writer) error {
	j, err := loadJournal(home)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		report(out, "%v; %s was kept for inspection", err, journalPath(home))
		return err
	}
	if j.Phase == phaseCompletion {
		// The transaction completed; at most staging remnants remain.
		_ = os.RemoveAll(j.Stage)
		_ = os.RemoveAll(j.Candidate)
		_ = os.Remove(journalPath(home))
		return nil
	}
	changed := journalChanged(j)
	if j.Stage == "" || dirGone(j.Stage) {
		_ = os.RemoveAll(j.Candidate)
		_ = os.Remove(journalPath(home))
		if changed {
			report(out, "the interrupted upgrade to %s kept its installed state (staged rollback data in %s is gone)", j.Version, j.Stage)
		}
		return nil
	}
	if changed {
		report(out, "rolling back the interrupted upgrade to %s (interrupted in %s)", j.Version, j.Phase)
	}
	kept, rerr := rollbackJournal(home, j)
	if len(kept) > 0 {
		report(out, "staged rollback data kept in %s", j.Stage)
	} else {
		_ = os.RemoveAll(j.Stage)
	}
	if rerr != nil {
		// The rollback is idempotent; keep the journal so the next
		// invocation retries it.
		report(out, "the rollback of the interrupted upgrade is incomplete: %v", rerr)
		return rerr
	}
	_ = os.Remove(journalPath(home))
	return nil
}

func dirGone(p string) bool {
	fi, err := os.Stat(p)
	return err != nil || !fi.IsDir()
}

func report(out io.Writer, format string, a ...any) {
	if out != nil {
		fmt.Fprintf(out, "zeze: "+format+"\n", a...)
	}
}

// repairUpgrade repairs an interrupted upgrade transaction before an ordinary
// command is dispatched. A journal owned by a live process belongs to a
// running transaction and is never touched; otherwise the upgrade lock
// serializes the repair against a starting transaction. Everything here is
// best-effort: a repair failure is reported but never blocks the command.
func repairUpgrade(home string, out io.Writer) {
	j, err := loadJournal(home)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			report(out, "%v; %s was kept for inspection", err, journalPath(home))
		}
		return
	}
	if j.PID > 0 && j.PID != os.Getpid() && paths.ProcessAlive(j.PID) {
		return // the transaction is still running
	}
	unlock, err := paths.Lock(home, "upgrade")
	if err != nil {
		return // an upgrade holds the lock
	}
	defer unlock()
	_ = repairJournal(home, out)
}

// repairUpgradeLocked is the same repair for a caller that already holds the
// upgrade lock (the upgrade transaction itself). The lock is the authority
// here: whatever journal exists was interrupted.
func repairUpgradeLocked(home string, out io.Writer) error {
	return repairJournal(home, out)
}
