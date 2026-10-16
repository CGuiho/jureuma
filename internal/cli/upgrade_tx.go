package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/CGuiho/jureuma/internal/agentres"
	"github.com/CGuiho/jureuma/internal/config"
	"github.com/CGuiho/jureuma/internal/paths"
	"github.com/CGuiho/jureuma/internal/runner"
	"github.com/CGuiho/jureuma/internal/schema"
	"github.com/CGuiho/jureuma/internal/update"
	"github.com/CGuiho/jureuma/internal/xerr"
	"github.com/spf13/cobra"
)

// The synchronous upgrade transaction (convention: Synchronous Upgrade
// Transaction). It mirrors install.sh: stage and verify every artifact of the
// selected release, build the immutable versions/<v>/ directory, replace the
// agent skill projections, switch current.json, verify through the stable
// launcher, and roll everything back when a step after the first change fails.

const (
	currentFile   = "current.json"
	installedFile = "installed-artifacts.json"
	selfTestCmd   = "__self-test"
)

var (
	semverPattern = `(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?`
	currentRe     = regexp.MustCompile(`^\{"protocol":1,"active":"(` + semverPattern + `)","previous":(null|"(` + semverPattern + `)")\}\n?$`)
)

// currentPointer is current.json.
type currentPointer struct{ Active, Previous string }

func readCurrent(home string) (currentPointer, bool) {
	data, err := os.ReadFile(filepath.Join(home, currentFile))
	if err != nil || len(data) > 4096 {
		return currentPointer{}, false
	}
	m := currentRe.FindStringSubmatch(string(data))
	if m == nil {
		return currentPointer{}, false
	}
	// Group 1 is active; the previous version is the group after "null|".
	prev := ""
	if idx := strings.Index(string(data), `"previous":"`); idx >= 0 {
		rest := string(data)[idx+len(`"previous":"`):]
		prev = rest[:strings.IndexByte(rest, '"')]
	}
	return currentPointer{Active: m[1], Previous: prev}, true
}

func (c currentPointer) marshal() []byte {
	prev := "null"
	if c.Previous != "" {
		prev = `"` + c.Previous + `"`
	}
	return []byte(`{"protocol":1,"active":"` + c.Active + `","previous":` + prev + "}\n")
}

// upgradeTx holds the state of one upgrade transaction. The rollback state
// lives in the write-ahead journal on disk, not in memory, so an interrupted
// transaction can be repaired completely by a later invocation.
type upgradeTx struct {
	cmd                *cobra.Command
	home, bin, stage   string
	version, tag       string
	asset, payloadName string
	launcher           string

	j         *upgradeJournal
	keepStage bool     // a rollback left staged backups behind
	retired   []string // skill IDs this release removes
}

// runUpgrade performs the transaction for rel. force allows terminating live
// run instances just before activation.
func (a *app) runUpgrade(cmd *cobra.Command, home string, rel update.Release, force bool) error {
	bin, err := paths.BinDir()
	if err != nil {
		return xerr.Failure("cannot resolve ~/.guiho/bin")
	}
	tempRoot, err := paths.TempDir()
	if err != nil {
		return xerr.Failure("cannot resolve ~/.guiho/.temp")
	}
	if !update.ValidTag(rel.Tag) {
		return xerr.Failure("the release tag contains unexpected characters")
	}
	cur, installed := readCurrent(home)
	launcher := filepath.Join(bin, exeName("jureuma"))
	if !installed {
		return xerr.Failure("no installer-managed installation was found (%s is missing or invalid); run the reinstall command below", filepath.Join(home, currentFile))
	}
	if fi, err := os.Lstat(launcher); err != nil || !fi.Mode().IsRegular() {
		return xerr.Failure("the stable launcher %s is missing; run the reinstall command below", launcher)
	}

	unlock, err := paths.Lock(home, "upgrade")
	if err != nil {
		return xerr.Failure("another jureuma upgrade is running")
	}
	defer unlock()
	// With the lock held no transaction can be live, so whatever journal
	// exists was interrupted and must be repaired before this transaction
	// may overwrite it.
	if err := repairUpgradeLocked(home, cmd.ErrOrStderr()); err != nil {
		return xerr.Failure("the interrupted upgrade transaction could not be repaired; run the reinstall command below")
	}

	if err := os.MkdirAll(tempRoot, 0o755); err != nil {
		return xerr.Failure("cannot create %s", tempRoot)
	}
	stage, err := os.MkdirTemp(tempRoot, "jureuma-upgrade-")
	if err != nil {
		return xerr.Failure("cannot create the staging directory")
	}
	if filepath.Dir(stage) != tempRoot || !strings.HasPrefix(filepath.Base(stage), "jureuma-upgrade-") {
		os.RemoveAll(stage)
		return xerr.Failure("the staging directory is outside ~/.guiho/.temp")
	}

	tx := &upgradeTx{
		cmd: cmd, home: home, bin: bin, stage: stage,
		version: rel.Version, tag: rel.Tag,
		asset:       update.PayloadAsset(runtime.GOOS, runtime.GOARCH),
		payloadName: exeName("jureuma"),
		launcher:    launcher,
	}
	tx.j = newUpgradeJournal(rel.Version, "", stage, filepath.Join(home, "versions", ".candidate-"+filepath.Base(stage)))
	defer func() {
		if !tx.keepStage {
			os.RemoveAll(stage)
		}
	}()

	// The version being superseded: the launcher records it as previous, and
	// every other process running its payload must be terminated before
	// activation.
	next := currentPointer{Active: tx.version}
	prev := cur.Active
	if prev == tx.version {
		prev = cur.Previous
	}
	if prev != "" && fileExists(filepath.Join(home, "versions", prev, tx.payloadName)) {
		next.Previous = prev
	}
	tx.j.Previous = prev

	// From here on, every failure rolls back through the journal.
	committed := false
	defer func() {
		if !committed {
			tx.failRollback()
		}
	}()
	if err := tx.saveJournal(); err != nil {
		return err
	}

	man, files, err := tx.download()
	if err != nil {
		return err
	}
	if err := tx.checkCandidate(filepath.Join(stage, "dl", tx.asset)); err != nil {
		return err
	}
	if err := tx.planArtifacts(man); err != nil {
		return err
	}
	if err := tx.buildVersion(man, files); err != nil {
		return err
	}
	if err := tx.replaceSkills(); err != nil {
		return err
	}
	if err := tx.writeManifest(files["artifacts.json"]); err != nil {
		return err
	}

	// Terminate live run instances only now, right before activation.
	if inst := runner.Instances(home); len(inst) > 0 {
		if !force {
			return xerr.Failure("%d jureuma run %s alive; stop them or pass --force", len(inst), plural(len(inst), "instance is", "instances are"))
		}
		for _, i := range inst {
			if p, err := os.FindProcess(i.PID); err == nil {
				_ = p.Signal(os.Interrupt)
			}
		}
	}
	// Every other process owned by the current user whose verified
	// executable path is the previous payload is terminated before
	// activation; a survivor fails the upgrade without activating.
	if cur.Active != "" {
		oldPayload := filepath.Join(home, "versions", cur.Active, tx.payloadName)
		if err := runner.TerminateOldPayload(home, oldPayload); err != nil {
			return xerr.Failure("%v; the new release was not activated", err)
		}
	}

	if err := tx.planActivation(); err != nil {
		return err
	}
	if err := tx.activate(next); err != nil {
		return err
	}
	if err := tx.savePhase(phaseVerification); err != nil {
		return err
	}
	if err := tx.verify(); err != nil {
		return err
	}
	if err := tx.applyInstruction(); err != nil {
		return err
	}
	if err := tx.finish(); err != nil {
		return err
	}
	committed = true

	// Disposable state and obsolete payloads (keep the active and previous).
	_ = os.Remove(filepath.Join(home, update.CacheFile))
	if ents, err := os.ReadDir(filepath.Join(home, "versions")); err == nil {
		for _, e := range ents {
			v := e.Name()
			if _, ok := update.Parse(v); !ok || !e.IsDir() || v == next.Active || v == next.Previous {
				continue
			}
			if os.RemoveAll(filepath.Join(home, "versions", v)) == nil {
				outf(cmd, "removed obsolete payload %s\n", v)
			}
		}
	}

	outf(cmd, "[OK] upgraded jureuma %s -> %s\n", a.version, tx.version)
	outf(cmd, "  launcher:  %s\n", launcher)
	outf(cmd, "  payload:   %s\n", filepath.Join(home, "versions", tx.version, tx.payloadName))
	if next.Previous != "" {
		outf(cmd, "  rollback:  %s (inactive)\n", next.Previous)
	}
	for _, t := range tx.j.SkillWrites {
		outf(cmd, "  skill:     %s\n", t)
	}
	for _, id := range tx.retired {
		outf(cmd, "  removed retired skill %s\n", id)
	}
	if tx.j.AgentsTarget != "" {
		outf(cmd, "  AGENTS.md: %s (jureuma block)\n", tx.j.AgentsTarget)
	}
	outf(cmd, "verified: %s --version reports %s\n", launcher, tx.version)
	outf(cmd, "next: run jureuma init, then jureuma --version\n")
	return nil
}

// download fetches and verifies every artifact of the release into
// stage/dl. It returns the manifest and the downloaded files by asset name.
func (tx *upgradeTx) download() (*update.Manifest, map[string]string, error) {
	dl := filepath.Join(tx.stage, "dl")
	if err := os.MkdirAll(dl, 0o700); err != nil {
		return nil, nil, xerr.Failure("cannot create the staging directory")
	}
	ctx := context.Background()
	files := map[string]string{}
	get := func(asset string) error {
		dst := filepath.Join(dl, asset)
		if err := update.Download(ctx, tx.tag, asset, dst); err != nil {
			return xerr.Failure("%v; nothing was changed", err)
		}
		files[asset] = dst
		return nil
	}
	outf(tx.cmd, "downloading jureuma %s...\n", tx.version)
	if err := get("checksums.txt"); err != nil {
		return nil, nil, err
	}
	data, err := os.ReadFile(files["checksums.txt"])
	if err != nil {
		return nil, nil, xerr.Failure("cannot read checksums.txt")
	}
	sums, err := update.ParseChecksums(data)
	if err != nil {
		return nil, nil, xerr.Failure("%v; nothing was changed", err)
	}
	verify := func(asset string) error {
		want, ok := sums[asset]
		if !ok {
			return xerr.Failure("checksums.txt has no entry for %s; nothing was changed", asset)
		}
		got, err := fileSHA256(files[asset])
		if err != nil || got != want {
			return xerr.Failure("SHA-256 mismatch for %s; nothing was changed", asset)
		}
		return nil
	}
	if err := get("artifacts.json"); err != nil {
		return nil, nil, err
	}
	if err := verify("artifacts.json"); err != nil {
		return nil, nil, err
	}
	mdata, err := os.ReadFile(files["artifacts.json"])
	if err != nil {
		return nil, nil, xerr.Failure("cannot read artifacts.json")
	}
	man, err := update.ParseManifest(mdata)
	if err != nil {
		return nil, nil, xerr.Failure("%v; nothing was changed", err)
	}
	if man.Version != tx.version {
		return nil, nil, xerr.Failure("artifacts.json declares version %s, expected %s; nothing was changed", man.Version, tx.version)
	}
	declared := map[string]bool{}
	for _, art := range man.ForPlatform(runtime.GOOS, runtime.GOARCH) {
		declared[art.Asset] = true
	}
	for _, need := range append([]string{tx.asset, update.LauncherAsset(runtime.GOOS, runtime.GOARCH)}, update.AgentAssets...) {
		if !declared[need] {
			return nil, nil, xerr.Failure("artifacts.json does not declare %s; nothing was changed", need)
		}
	}
	for _, art := range man.ForPlatform(runtime.GOOS, runtime.GOARCH) {
		if _, done := files[art.Asset]; !done {
			if err := get(art.Asset); err != nil {
				return nil, nil, err
			}
			if err := verify(art.Asset); err != nil {
				return nil, nil, err
			}
		}
		if strings.ToLower(art.SHA256) != sums[art.Asset] {
			return nil, nil, xerr.Failure("artifacts.json and checksums.txt disagree about %s; nothing was changed", art.Asset)
		}
	}
	outf(tx.cmd, "[OK] SHA-256 verification complete\n")
	return man, files, nil
}

// runQuiet runs exe with args (update check disabled) and returns stdout.
func runQuiet(dir, exe string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	c := exec.CommandContext(ctx, exe, args...)
	c.Dir = dir
	c.Env = append(os.Environ(), update.DisableEnv+"=1")
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = io.Discard
	err := c.Run()
	return strings.TrimSpace(out.String()), err
}

// checkCandidate runs the staged payload: --version must be the exact target
// and the hidden self-test must pass.
func (tx *upgradeTx) checkCandidate(payload string) error {
	if err := os.Chmod(payload, 0o755); err != nil {
		return xerr.Failure("cannot prepare the downloaded payload")
	}
	got, err := runQuiet(tx.stage, payload, "--version")
	if err != nil {
		return xerr.Failure("the downloaded payload failed to run; nothing was changed")
	}
	if got != tx.version {
		return xerr.Failure("the downloaded payload reports version %q, expected %s; nothing was changed", got, tx.version)
	}
	if _, err := runQuiet(tx.stage, payload, selfTestCmd); err != nil {
		return xerr.Failure("the downloaded payload failed its self-test; nothing was changed")
	}
	outf(tx.cmd, "[OK] payload self-test passed (%s)\n", tx.version)
	return nil
}

// buildVersion builds versions/<v>/ beside its final place and renames it in,
// following the write-ahead plan in the journal. Launcher artifacts are not
// installed here: the stable launcher lives in the shared ~/.guiho/bin
// directory and ordinary upgrades never replace it.
func (tx *upgradeTx) buildVersion(man *update.Manifest, files map[string]string) error {
	versions := filepath.Join(tx.home, "versions")
	if err := os.MkdirAll(versions, 0o755); err != nil {
		return xerr.Failure("cannot create %s", versions)
	}
	cand := tx.j.Candidate
	defer os.RemoveAll(cand)
	prefix := "versions/" + tx.version + "/"
	for _, art := range man.ForPlatform(runtime.GOOS, runtime.GOARCH) {
		if art.Kind == "launcher" {
			continue
		}
		rel := strings.TrimPrefix(art.InstalledPath, prefix)
		dst := filepath.Join(cand, filepath.FromSlash(rel))
		var err error
		switch {
		case art.Kind == "binary":
			dst = filepath.Join(cand, tx.payloadName)
			err = copyFile(files[art.Asset], dst, 0o555)
		case art.Kind == "skill" && strings.HasSuffix(art.Asset, ".zip"):
			err = unzipSkill(files[art.Asset], path.Base(art.InstalledPath), dst)
		default:
			err = copyFile(files[art.Asset], dst, 0o444)
		}
		if err != nil {
			return xerr.Failure("cannot install %s: %v", art.Asset, err)
		}
	}
	for _, f := range []string{"artifacts.json", "checksums.txt"} {
		if err := copyFile(files[f], filepath.Join(cand, "artifacts", f), 0o444); err != nil {
			return xerr.Failure("cannot install %s", f)
		}
	}
	if !fileExists(filepath.Join(cand, tx.payloadName)) {
		return xerr.Failure("artifacts.json does not install the payload")
	}
	final := filepath.Join(versions, tx.version)
	if tx.j.VersionBackup != "" {
		if err := os.Rename(final, tx.j.VersionBackup); err != nil {
			return xerr.Failure("cannot replace %s", final)
		}
	}
	if err := os.Rename(cand, final); err != nil {
		return xerr.Failure("cannot install %s", final)
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}

// unzipSkill extracts the archive's single top-level directory id into dst.
// Every entry must stay inside id/.
func unzipSkill(src, id, dst string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return errors.New("not a zip archive")
	}
	defer zr.Close()
	found := false
	for _, f := range zr.File {
		name := f.Name
		if strings.Contains(name, `\`) || path.IsAbs(name) || !strings.HasPrefix(name, id+"/") {
			return errors.New("unexpected path in the archive")
		}
		rel := strings.TrimPrefix(name, id+"/")
		if rel == "" {
			continue
		}
		if f.FileInfo().IsDir() {
			continue
		}
		if path.Clean(rel) != rel || rel == ".." || strings.HasPrefix(rel, "../") {
			return errors.New("unexpected path in the archive")
		}
		if !f.Mode().IsRegular() {
			return errors.New("unexpected entry type in the archive")
		}
		if rel == "SKILL.md" {
			found = true
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		target := filepath.Join(dst, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			rc.Close()
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, io.LimitReader(rc, update.MaxAssetSize))
		rc.Close()
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
	if !found {
		return errors.New("the archive has no SKILL.md")
	}
	return nil
}

// skillIDs returns the skill IDs a manifest installs.
func skillIDs(man *update.Manifest) []string {
	var out []string
	if man == nil {
		return nil
	}
	for _, a := range man.Artifacts {
		if a.Kind == "skill" {
			if id := path.Base(a.InstalledPath); validSkillID(id) {
				out = append(out, id)
			}
		}
	}
	return out
}

func validSkillID(id string) bool {
	return id != "" && id != "." && id != ".." && !strings.HasPrefix(id, ".") && !strings.ContainsAny(id, `/\:`)
}

// planArtifacts records the write-ahead plan for every artifact change the
// transaction is about to make (immutable version directory, skill
// projections, installed-artifacts.json) and stages the backups its rollback
// needs. It runs before the first change outside the transaction's staging,
// so an interruption can never leave an unrecorded change behind.
func (tx *upgradeTx) planArtifacts(man *update.Manifest) error {
	j := tx.j
	j.Phase = phaseArtifactReplace
	final := filepath.Join(tx.home, "versions", tx.version)
	if _, err := os.Lstat(final); err == nil {
		j.VersionBackup = filepath.Join(tx.stage, "backup-version")
	}
	// versions/<v> did not exist when this plan was written (or is restored
	// from VersionBackup), so whatever is there after a rollback point is the
	// transaction's own build. The upgrade lock serializes every jureuma-side
	// writer of that directory.
	j.CreatesVersion = true

	dests, err := skillDests()
	if err != nil {
		return err
	}
	var prev *update.Manifest
	if data, err := os.ReadFile(filepath.Join(tx.home, installedFile)); err == nil {
		prev, _ = update.ParseManifest(data)
	}
	ids := skillIDs(man)
	keep := map[string]bool{}
	for _, id := range ids {
		keep[id] = true
	}
	for _, id := range skillIDs(prev) {
		if !keep[id] {
			tx.retired = append(tx.retired, id)
		}
	}
	n := 0
	for _, root := range dests {
		for _, id := range append(append([]string{}, ids...), tx.retired...) {
			target := filepath.Join(root, id)
			if _, err := os.Lstat(target); err == nil {
				n++
				j.SkillBackups[target] = filepath.Join(tx.stage, fmt.Sprintf("backup-skill-%d", n))
			}
		}
		for _, id := range ids {
			j.SkillWrites = append(j.SkillWrites, filepath.Join(root, id))
		}
	}

	// The manifest backup is staged before the journal records it: a
	// recorded backup always exists, so its restore never loses the file.
	if data, err := os.ReadFile(filepath.Join(tx.home, installedFile)); err == nil {
		bak := filepath.Join(tx.stage, "backup-manifest.json")
		if err := os.WriteFile(bak, data, 0o600); err != nil {
			return xerr.Failure("cannot stage the installed-artifacts.json backup")
		}
		j.ManifestBackup = bak
	}
	j.WritesManifest = true
	return tx.saveJournal()
}

// replaceSkills replaces the managed skill projections with the release's
// canonical copies and removes skills the previous release had but this one
// does not. Every replaced or removed directory is moved into staging first,
// exactly as the journal's write-ahead plan records it.
func (tx *upgradeTx) replaceSkills() error {
	for _, target := range sortedKeys(tx.j.SkillBackups) {
		if _, err := os.Lstat(target); err == nil {
			if err := os.Rename(target, tx.j.SkillBackups[target]); err != nil {
				return xerr.Failure("cannot replace %s", target)
			}
		}
	}
	for _, target := range tx.j.SkillWrites {
		src := filepath.Join(tx.home, "versions", tx.version, "artifacts", "skills", filepath.Base(target))
		if err := copyTree(src, target); err != nil {
			return xerr.Failure("cannot install %s", target)
		}
	}
	return nil
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		return copyFile(p, target, 0o644)
	})
}

// writeManifest installs the release's artifacts.json. Its rollback backup is
// already staged and recorded by planArtifacts.
func (tx *upgradeTx) writeManifest(src string) error {
	dst := filepath.Join(tx.home, installedFile)
	data, err := os.ReadFile(src)
	if err != nil {
		return xerr.Failure("cannot read artifacts.json")
	}
	if err := paths.AtomicWrite(dst, data, 0o644); err != nil {
		return xerr.Failure("cannot write %s", dst)
	}
	return nil
}

// planActivation stages the current.json backup and records the write-ahead
// plan before the atomic pointer swap.
func (tx *upgradeTx) planActivation() error {
	j := tx.j
	j.Phase = phaseActivation
	if data, err := os.ReadFile(filepath.Join(tx.home, currentFile)); err == nil {
		bak := filepath.Join(tx.stage, "backup-current.json")
		if err := os.WriteFile(bak, data, 0o600); err != nil {
			return xerr.Failure("cannot stage the current.json backup")
		}
		j.CurrentBackup = bak
	}
	j.WritesCurrent = true
	return tx.saveJournal()
}

// activate writes current.json atomically so the stable launcher selects the
// new immutable payload. The launcher itself is never replaced here: it is a
// released binary owned by the installers, and ordinary upgrades must not
// replace the running launcher.
func (tx *upgradeTx) activate(next currentPointer) error {
	p := filepath.Join(tx.home, currentFile)
	if err := paths.AtomicWrite(p, next.marshal(), 0o644); err != nil {
		return xerr.Failure("cannot write %s", p)
	}
	return nil
}

// verify runs the stable launcher and the installed payload's self-test.
func (tx *upgradeTx) verify() error {
	got, err := runQuiet(tx.stage, tx.launcher, "--version")
	if err != nil {
		return xerr.Failure("the launcher failed to run the new payload")
	}
	if got != tx.version {
		return xerr.Failure("the launcher reports version %q, expected %s", got, tx.version)
	}
	if _, err := runQuiet(tx.stage, filepath.Join(tx.home, "versions", tx.version, tx.payloadName), selfTestCmd); err != nil {
		return xerr.Failure("the installed payload failed its self-test")
	}
	outf(tx.cmd, "[OK] activated %s through %s\n", tx.version, tx.launcher)
	return nil
}

// applyInstruction refreshes the AGENTS.md block with the new payload when
// run from a project root (the same rule as install.sh). Its rollback backup
// is staged and recorded before the child command writes anything.
func (tx *upgradeTx) applyInstruction() error {
	wd, err := os.Getwd()
	if err != nil {
		return nil
	}
	agents := filepath.Join(wd, "AGENTS.md")
	if !fileExists(filepath.Join(wd, "jureuma.yaml")) && !(fileExists(agents) && fileExists(filepath.Join(wd, ".git"))) {
		return nil
	}
	tx.j.AgentsTarget = agents
	if data, err := os.ReadFile(agents); err == nil {
		bak := filepath.Join(tx.stage, "backup-agents.md")
		if err := os.WriteFile(bak, data, 0o600); err != nil {
			return xerr.Failure("cannot stage the AGENTS.md backup")
		}
		tx.j.AgentsBackup, tx.j.AgentsHad = bak, true
	}
	if err := tx.saveJournal(); err != nil {
		return err
	}
	if _, err := runQuiet(wd, tx.launcher, "agent", "instruction", "apply"); err != nil {
		return xerr.Failure("could not apply the jureuma block to %s", agents)
	}
	return nil
}

// savePhase records the next transaction phase in the journal.
func (tx *upgradeTx) savePhase(phase string) error {
	tx.j.Phase = phase
	return tx.saveJournal()
}

// saveJournal persists the write-ahead transaction record.
func (tx *upgradeTx) saveJournal() error {
	if err := tx.j.save(tx.home); err != nil {
		return xerr.Failure("cannot write the upgrade transaction journal")
	}
	return nil
}

// failRollback completes the rollback of a failed transaction through the
// journal and removes the journal when the rollback finished. When staged
// backups could not be restored they are kept: they are the only copies, and
// the next invocation's repair retries the rollback from them.
func (tx *upgradeTx) failRollback() {
	if journalChanged(tx.j) {
		errf(tx.cmd, "rolling back to the previous installation...\n")
	}
	tx.j.Phase = phaseRollback
	_ = tx.j.save(tx.home)
	kept, err := rollbackJournal(tx.home, tx.j)
	if len(kept) > 0 {
		tx.keepStage = true
		errf(tx.cmd, "jureuma: staged rollback data kept in %s\n", tx.j.Stage)
	}
	if err != nil {
		errf(tx.cmd, "jureuma: the rollback is incomplete: %v\n", err)
		return // the journal stays; the next invocation repairs
	}
	_ = os.Remove(journalPath(tx.home))
}

// finish marks the transaction complete and removes its journal. A journal
// that cannot be completed and removed fails the transaction: a later
// invocation must never mistake a finished upgrade for an interrupted one.
func (tx *upgradeTx) finish() error {
	if err := tx.savePhase(phaseCompletion); err != nil {
		return err
	}
	if err := os.Remove(journalPath(tx.home)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return xerr.Failure("cannot remove %s", journalPath(tx.home))
	}
	return nil
}

// ---- hidden self-test ----

// selfTest verifies that this payload can start and read every embedded
// release resource. It changes nothing on disk.
func selfTest() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("embedded resources are unreadable")
		}
	}()
	main, ok := agentres.SkillByID(agentres.MainSkillID)
	if !ok || len(main.Files["SKILL.md"]) == 0 {
		return errors.New("the main skill is missing")
	}
	for _, id := range []string{agentres.MainPromptID, agentres.InstallPromptID, agentres.UninstallPromptID} {
		if p, ok := agentres.PromptByID(id); !ok || strings.TrimSpace(p.Body) == "" {
			return fmt.Errorf("prompt %s is missing", id)
		}
	}
	if strings.TrimSpace(agentres.Instruction()) == "" {
		return errors.New("the agent instruction is empty")
	}
	if err := selfTestConfig(); err != nil {
		return err
	}
	var v any
	for _, data := range selfTestSchemas() {
		if err := json.Unmarshal(data, &v); err != nil {
			return errors.New("an embedded configuration schema is not JSON")
		}
	}
	return nil
}

func (a *app) cmdSelfTest() *cobra.Command {
	return &cobra.Command{
		Use:    selfTestCmd,
		Short:  "verify the embedded release resources (used by the installers and upgrade)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := selfTest(); err != nil {
				return xerr.Failure("self-test failed: %v", err)
			}
			outf(cmd, "self-test passed (%s)\n", a.version)
			return nil
		},
	}
}

func selfTestSchemas() [2][]byte {
	return [2][]byte{config.ProjectSchemaJSON(), config.GlobalSchemaJSON()}
}

// selfTestConfig parses the embedded examples against the embedded schemas
// and compiles the starter configuration schema.
func selfTestConfig() error {
	if _, err := config.ParseProject(config.ProjectExampleYAML()); err != nil {
		return errors.New("the embedded jureuma.yaml example does not validate")
	}
	if _, err := config.ParseGlobal(config.GlobalExampleYAML()); err != nil {
		return errors.New("the embedded global example does not validate")
	}
	if _, err := schema.Compile(schema.Starter(), "starter.schema.json"); err != nil {
		return errors.New("the starter schema does not compile")
	}
	return nil
}
