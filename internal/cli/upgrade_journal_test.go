package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/CGuiho/zeze/internal/paths"
	"github.com/CGuiho/zeze/internal/update"
)

// journalFixture builds an installer-managed home with a staging directory
// and an interrupted upgrade transaction journal.
type journalFixture struct {
	root         string // the user home
	home         string // <root>/.guiho/zeze
	stage        string
	candidate    string
	versionDir   string // <home>/versions/0.2.0
	skillTarget  string
	manifestPath string
	currentPath  string
	agentsPath   string
	j            *upgradeJournal
}

func newJournalFixture(t *testing.T) *journalFixture {
	t.Helper()
	root := t.TempDir()
	t.Setenv("ZEZE_USER_HOME", root)
	t.Setenv("HOME", root)
	t.Setenv("USERPROFILE", root)
	f := &journalFixture{
		root:         root,
		home:         filepath.Join(root, ".guiho", "zeze"),
		stage:        filepath.Join(root, ".guiho", ".temp", "zeze-upgrade-test"),
		manifestPath: filepath.Join(root, ".guiho", "zeze", installedFile),
		currentPath:  filepath.Join(root, ".guiho", "zeze", currentFile),
	}
	f.candidate = filepath.Join(f.home, "versions", ".candidate-zeze-upgrade-test")
	f.versionDir = filepath.Join(f.home, "versions", "0.2.0")
	for _, d := range []string{filepath.Join(f.home, "state"), f.stage} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	dests, err := skillDests()
	if err != nil || len(dests) == 0 {
		t.Fatalf("skill destinations: %v %v", dests, err)
	}
	f.skillTarget = filepath.Join(dests[0], "guiho-s-zeze")
	f.j = newUpgradeJournal("0.2.0", "0.1.0", f.stage, f.candidate)
	return f
}

func (f *journalFixture) write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (f *journalFixture) read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func (f *journalFixture) save(t *testing.T) {
	t.Helper()
	if err := f.j.save(f.home); err != nil {
		t.Fatal(err)
	}
}

func (f *journalFixture) gone(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s must be gone, got %v", path, err)
	}
}

func (f *journalFixture) journalGone(t *testing.T) {
	t.Helper()
	f.gone(t, journalPath(f.home))
}

// interrupted writes the state an upgrade leaves behind when it is killed
// after replacing every artifact: new payload, skills, manifest and pointer,
// with the previous versions staged as backups.
func (f *journalFixture) interrupted(t *testing.T) {
	t.Helper()
	f.write(t, filepath.Join(f.stage, "backup-version", "zeze"), "old-payload")
	f.write(t, filepath.Join(f.stage, "backup-manifest.json"), "old-manifest")
	f.write(t, filepath.Join(f.stage, "backup-current.json"), "old-current")
	f.write(t, filepath.Join(f.stage, "backup-agents.md"), "old-agents")
	f.write(t, filepath.Join(f.stage, "backup-skill-1", "SKILL.md"), "old-skill")

	f.j.VersionBackup = filepath.Join(f.stage, "backup-version")
	f.j.CreatesVersion = true
	f.j.SkillBackups = map[string]string{f.skillTarget: filepath.Join(f.stage, "backup-skill-1")}
	f.j.SkillWrites = []string{f.skillTarget}
	f.j.WritesManifest = true
	f.j.ManifestBackup = filepath.Join(f.stage, "backup-manifest.json")
	f.j.WritesCurrent = true
	f.j.CurrentBackup = filepath.Join(f.stage, "backup-current.json")
	f.j.AgentsTarget = f.agentsPath
	f.j.AgentsHad = true
	f.j.AgentsBackup = filepath.Join(f.stage, "backup-agents.md")

	f.write(t, filepath.Join(f.versionDir, "zeze"), "new-payload")
	f.write(t, filepath.Join(f.skillTarget, "SKILL.md"), "new-skill")
	f.write(t, f.manifestPath, "new-manifest")
	f.write(t, f.currentPath, "new-current")
	f.write(t, f.agentsPath, "new-agents")
}

// assertRestored verifies the state before the transaction.
func (f *journalFixture) assertRestored(t *testing.T) {
	t.Helper()
	for path, want := range map[string]string{
		filepath.Join(f.versionDir, "zeze"):      "old-payload",
		filepath.Join(f.skillTarget, "SKILL.md"): "old-skill",
		f.manifestPath:                           "old-manifest",
		f.currentPath:                            "old-current",
		f.agentsPath:                             "old-agents",
	} {
		if got := f.read(t, path); got != want {
			t.Fatalf("%s = %q, want %q", path, got, want)
		}
	}
	f.gone(t, f.stage)
	f.journalGone(t)
}

func TestJournalRoundTrip(t *testing.T) {
	f := newJournalFixture(t)
	f.j.Phase = phaseArtifactReplace
	f.j.CreatesVersion = true
	f.j.SkillBackups = map[string]string{f.skillTarget: filepath.Join(f.stage, "backup-skill-1")}
	f.j.SkillWrites = []string{f.skillTarget}
	f.save(t)
	j, err := loadJournal(f.home)
	if err != nil {
		t.Fatal(err)
	}
	if j.Phase != phaseArtifactReplace || j.Version != "0.2.0" || j.Previous != "0.1.0" ||
		j.Stage != f.stage || j.Candidate != f.candidate || !j.CreatesVersion ||
		j.SkillBackups[f.skillTarget] != filepath.Join(f.stage, "backup-skill-1") ||
		len(j.SkillWrites) != 1 {
		t.Fatalf("round trip lost fields: %+v", j)
	}
	if !journalChanged(j) {
		t.Fatal("a plan with changes must report changed")
	}
}

func TestLoadJournalRefusesUnsafeJournals(t *testing.T) {
	cases := map[string]func(f *journalFixture){
		"stage outside the temp root": func(f *journalFixture) {
			f.j.Stage = filepath.Join(f.home, "state", "elsewhere")
		},
		"stage without the staging prefix": func(f *journalFixture) {
			f.j.Stage = filepath.Join(filepath.Dir(f.stage), "other-dir")
		},
		"candidate outside versions/": func(f *journalFixture) {
			f.j.Candidate = filepath.Join(f.stage, ".candidate-x")
		},
		"backup outside the staging directory": func(f *journalFixture) {
			f.j.ManifestBackup = filepath.Join(f.home, "backup.json")
		},
		"skill target outside the skill destinations": func(f *journalFixture) {
			f.j.SkillWrites = []string{filepath.Join(f.home, "state", "guiho-s-zeze")}
		},
		"skill backup outside the staging directory": func(f *journalFixture) {
			f.j.SkillBackups = map[string]string{f.skillTarget: filepath.Join(f.root, "backup-skill")}
		},
		"unexpected instruction target": func(f *journalFixture) {
			f.j.AgentsTarget = filepath.Join(f.root, "important.txt")
		},
		"unknown phase": func(f *journalFixture) {
			f.j.Phase = "nonsense"
		},
		"invalid version": func(f *journalFixture) {
			f.j.Version = "../../etc"
		},
		"invalid previous version": func(f *journalFixture) {
			f.j.Previous = "not-a-version"
		},
		"unsupported schema": func(f *journalFixture) {
			f.j.Schema = 2
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := newJournalFixture(t)
			mutate(f)
			f.save(t)
			if _, err := loadJournal(f.home); err == nil {
				t.Fatal("an unsafe journal must be refused")
			}
			// Repair never acts on it and never deletes it.
			if err := repairJournal(f.home, nil); err == nil {
				t.Fatal("repair must report the unsafe journal")
			}
			if _, err := os.Stat(journalPath(f.home)); err != nil {
				t.Fatalf("the unsafe journal must be kept: %v", err)
			}
		})
	}
}

func TestRepairInterruptedPhases(t *testing.T) {
	for _, phase := range []string{phaseStaging, phaseArtifactReplace, phaseActivation, phaseVerification, phaseRollback} {
		t.Run(phase, func(t *testing.T) {
			f := newJournalFixture(t)
			f.agentsPath = filepath.Join(f.root, "project", "AGENTS.md")
			f.j.Phase = phase
			f.interrupted(t)
			f.save(t)
			if err := repairJournal(f.home, nil); err != nil {
				t.Fatal(err)
			}
			f.assertRestored(t)
			// Idempotent: a second repair finds nothing to do.
			if err := repairJournal(f.home, nil); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRepairAfterCompletionOnlyCleansUp(t *testing.T) {
	f := newJournalFixture(t)
	f.agentsPath = filepath.Join(f.root, "project", "AGENTS.md")
	f.j.Phase = phaseCompletion
	f.interrupted(t)
	f.save(t)
	if err := repairJournal(f.home, nil); err != nil {
		t.Fatal(err)
	}
	// The completed transaction's state is kept; only staging goes.
	if got := f.read(t, f.currentPath); got != "new-current" {
		t.Fatalf("current.json = %q, want the completed state", got)
	}
	f.gone(t, f.stage)
	f.journalGone(t)
}

func TestRepairLeavesTargetsWhoseBackupIsGone(t *testing.T) {
	f := newJournalFixture(t)
	f.j.Phase = phaseActivation
	f.j.CreatesVersion = true
	f.j.VersionBackup = filepath.Join(f.stage, "backup-version") // never staged
	f.j.WritesManifest = true
	f.j.ManifestBackup = filepath.Join(f.stage, "backup-manifest.json") // never staged
	f.j.WritesCurrent = true
	f.j.CurrentBackup = filepath.Join(f.stage, "backup-current.json") // never staged
	f.write(t, filepath.Join(f.versionDir, "zeze"), "new-payload")
	f.write(t, f.manifestPath, "new-manifest")
	f.write(t, f.currentPath, "new-current")
	f.save(t)
	if err := repairJournal(f.home, nil); err != nil {
		t.Fatal(err)
	}
	// A recorded backup that is gone was never written: the targets stand.
	for _, path := range []string{filepath.Join(f.versionDir, "zeze"), f.manifestPath, f.currentPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s must be left alone: %v", path, err)
		}
	}
	f.journalGone(t)
}

func TestRepairRemovesWhatTheTransactionCreated(t *testing.T) {
	f := newJournalFixture(t)
	f.j.Phase = phaseArtifactReplace
	f.j.CreatesVersion = true // versions/0.2.0 did not exist before
	f.j.SkillWrites = []string{f.skillTarget}
	f.write(t, filepath.Join(f.versionDir, "zeze"), "new-payload")
	f.write(t, filepath.Join(f.skillTarget, "SKILL.md"), "new-skill")
	f.write(t, f.manifestPath, "old-manifest") // not written by the transaction
	f.save(t)
	if err := repairJournal(f.home, nil); err != nil {
		t.Fatal(err)
	}
	f.gone(t, f.versionDir)
	f.gone(t, f.skillTarget)
	if got := f.read(t, f.manifestPath); got != "old-manifest" {
		t.Fatalf("manifest = %q, want untouched", got)
	}
	f.gone(t, f.stage)
	f.journalGone(t)
}

func TestRepairRestoresAReplacedVersionDirectory(t *testing.T) {
	f := newJournalFixture(t)
	f.j.Phase = phaseArtifactReplace
	f.j.VersionBackup = filepath.Join(f.stage, "backup-version")
	f.j.CreatesVersion = true
	f.write(t, filepath.Join(f.stage, "backup-version", "zeze"), "old-payload")
	f.write(t, filepath.Join(f.versionDir, "zeze"), "new-payload")
	f.save(t)
	if err := repairJournal(f.home, nil); err != nil {
		t.Fatal(err)
	}
	if got := f.read(t, filepath.Join(f.versionDir, "zeze")); got != "old-payload" {
		t.Fatalf("versions/0.2.0 = %q, want the staged original", got)
	}
	f.gone(t, f.stage)
	f.journalGone(t)
}

func TestRepairKeepsTheInstalledStateWhenStagingIsGone(t *testing.T) {
	f := newJournalFixture(t)
	f.agentsPath = filepath.Join(f.root, "project", "AGENTS.md")
	f.j.Phase = phaseActivation
	f.interrupted(t)
	f.save(t)
	if err := os.RemoveAll(f.stage); err != nil {
		t.Fatal(err)
	}
	if err := repairJournal(f.home, nil); err != nil {
		t.Fatal(err)
	}
	// Without the staged backups a piece-by-piece rollback could delete
	// pre-existing files: the installed state is kept instead.
	if got := f.read(t, f.currentPath); got != "new-current" {
		t.Fatalf("current.json = %q, want the kept state", got)
	}
	if got := f.read(t, filepath.Join(f.skillTarget, "SKILL.md")); got != "new-skill" {
		t.Fatalf("skill = %q, want the kept state", got)
	}
	f.journalGone(t)
}

// TestPlanArtifactsRecordsARollbackPlan drives the real write-ahead planner
// and verifies that a transaction interrupted right after it can be repaired
// completely from the journal.
func TestPlanArtifactsRecordsARollbackPlan(t *testing.T) {
	f := newJournalFixture(t)
	f.agentsPath = filepath.Join(f.root, "project", "AGENTS.md")
	// The pre-transaction state.
	f.write(t, filepath.Join(f.home, "versions", "0.2.0", "zeze"), "old-payload")
	f.write(t, filepath.Join(f.skillTarget, "SKILL.md"), "old-skill")
	f.write(t, f.manifestPath, "old-manifest")
	f.write(t, f.currentPath, "old-current")

	tx := &upgradeTx{
		home: f.home, stage: f.stage, version: "0.2.0", payloadName: "zeze",
		j: newUpgradeJournal("0.2.0", "0.1.0", f.stage, f.candidate),
	}
	man := &update.Manifest{
		Schema: 1, Name: "zeze", Version: "0.2.0",
		Artifacts: []update.Artifact{{
			ID: "guiho-s-zeze", Kind: "skill", Asset: "guiho-s-zeze.zip",
			OS: "any", Arch: "any",
			InstalledPath: "versions/0.2.0/artifacts/skills/guiho-s-zeze",
		}},
	}
	if err := tx.planArtifacts(man); err != nil {
		t.Fatal(err)
	}
	// Simulate the interrupted execution of the plan: stage the backups and
	// install the new artifacts, then "die" before the next phase.
	if err := os.Rename(f.versionDir, tx.j.VersionBackup); err != nil {
		t.Fatal(err)
	}
	f.write(t, filepath.Join(f.versionDir, "zeze"), "new-payload")
	if err := os.Rename(f.skillTarget, tx.j.SkillBackups[f.skillTarget]); err != nil {
		t.Fatal(err)
	}
	f.write(t, filepath.Join(f.skillTarget, "SKILL.md"), "new-skill")
	f.write(t, f.manifestPath, "new-manifest")

	if err := repairJournal(f.home, nil); err != nil {
		t.Fatal(err)
	}
	if got := f.read(t, filepath.Join(f.versionDir, "zeze")); got != "old-payload" {
		t.Fatalf("versions/0.2.0 = %q, want the original", got)
	}
	if got := f.read(t, filepath.Join(f.skillTarget, "SKILL.md")); got != "old-skill" {
		t.Fatalf("skill = %q, want the original", got)
	}
	if got := f.read(t, f.manifestPath); got != "old-manifest" {
		t.Fatalf("manifest = %q, want the original", got)
	}
	f.gone(t, f.stage)
	f.journalGone(t)
}

func TestRepairUpgradeSkipsALiveTransaction(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses the portable helper below only on the host")
	}
	f := newJournalFixture(t)
	f.j.Phase = phaseArtifactReplace
	f.j.CreatesVersion = true
	f.write(t, filepath.Join(f.versionDir, "zeze"), "new-payload")
	live := startLiveHelper(t)
	defer func() {
		_ = live.Process.Kill()
		_, _ = live.Process.Wait()
	}()
	f.j.PID = live.Process.Pid
	f.save(t)
	repairUpgrade(f.home, nil)
	// The live transaction is left alone.
	if got := f.read(t, filepath.Join(f.versionDir, "zeze")); got != "new-payload" {
		t.Fatalf("versions/0.2.0 = %q, want untouched", got)
	}
	if _, err := os.Stat(journalPath(f.home)); err != nil {
		t.Fatalf("the live journal must be kept: %v", err)
	}
	// When its owner is gone, the same journal is repaired.
	f.j.PID = deadPID(t)
	f.save(t)
	repairUpgrade(f.home, nil)
	f.gone(t, f.versionDir)
	f.journalGone(t)
}

// TestHelperLive holds a live process for TestRepairUpgradeSkipsALiveTransaction.
func TestHelperLive(t *testing.T) {
	if os.Getenv("ZEZE_TEST_LIVE") != "1" {
		t.Skip("helper for TestRepairUpgradeSkipsALiveTransaction")
	}
	for {
		time.Sleep(time.Hour)
	}
}

func startLiveHelper(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperLive")
	cmd.Env = append(os.Environ(), "ZEZE_TEST_LIVE=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	return cmd
}

// deadPID returns a PID of a process that has exited and been reaped, so the
// journal's liveness check sees its owner as gone.
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperExit")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	if paths.ProcessAlive(pid) {
		t.Skip("the exited helper's PID was reused")
	}
	return pid
}

func TestHelperExit(t *testing.T) {
	if os.Getenv("ZEZE_TEST_LIVE") == "1" {
		t.Fatal("wrong helper")
	}
}
