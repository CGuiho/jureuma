// Package runner injects a decrypted document into a child process and
// redacts the child's output.
//
// Run never prints configuration values itself. Every zeze-side failure
// returns exit code 125 with an error whose kind names the failure class.
package runner

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/CGuiho/zeze/internal/config"
	"github.com/CGuiho/zeze/internal/crypt"
	"github.com/CGuiho/zeze/internal/doc"
	"github.com/CGuiho/zeze/internal/keystore"
	"github.com/CGuiho/zeze/internal/paths"
	"github.com/CGuiho/zeze/internal/redact"
	"github.com/CGuiho/zeze/internal/schema"
	"github.com/CGuiho/zeze/internal/xerr"
)

// Formats.
const (
	FormatJSON    = "json"
	FormatFlatten = "flatten"
	FormatFile    = "file"
)

// DefaultEnvName is the variable that carries the document by default.
const DefaultEnvName = "ZEZE_CONFIG"

// MinRedactLen is the minimum length of a string value redacted by default.
const MinRedactLen = 4

// Options configures Run.
type Options struct {
	Doc             *doc.Value
	Schema          *schema.Schema
	Format          string
	EnvName, Prefix string
	Env             []string /* NAME=/pointer */
	Override        bool
	Redact          bool
	PrivateKeys     []crypt.PrivateKey
	Args            []string
	Stdin           io.Reader
	Stdout, Stderr  io.Writer
	Home            string

	// Environ is the inherited environment; nil means os.Environ().
	Environ []string
}

// runErr wraps err as a run failure (exit 125) keeping its kind.
func runErr(kind, format string, a ...any) error {
	e := xerr.Failure(format, a...).(*xerr.Error)
	e.Code = xerr.CodeRun
	e.Kind = kind
	return e
}

func asRun(err error) error {
	var e *xerr.Error
	if errors.As(err, &e) {
		return &xerr.Error{Code: xerr.CodeRun, Kind: e.Kind, Msg: e.Msg, Err: e.Err}
	}
	return &xerr.Error{Code: xerr.CodeRun, Kind: xerr.KindFailure, Err: err}
}

// envList is an ordered environment that follows the host's case rules.
type envList struct {
	raw   []string // entries without a usable name (Windows "=C:=..."), kept verbatim
	names []string
	vals  map[string]string
}

func newEnvList(environ []string) *envList {
	l := &envList{vals: map[string]string{}}
	for _, kv := range environ {
		i := strings.IndexByte(kv, '=')
		if i <= 0 {
			l.raw = append(l.raw, kv)
			continue
		}
		l.set(kv[:i], kv[i+1:])
	}
	return l
}

func (l *envList) has(name string) bool {
	_, ok := l.vals[envKey(name)]
	return ok
}

func (l *envList) set(name, val string) {
	k := envKey(name)
	if _, ok := l.vals[k]; !ok {
		l.names = append(l.names, name)
	}
	l.vals[k] = val
}

func (l *envList) environ() []string {
	out := append([]string(nil), l.raw...)
	for _, n := range l.names {
		out = append(out, n+"="+l.vals[envKey(n)])
	}
	return out
}

// ChildEnv returns environ without any private-key variable.
func ChildEnv(environ []string) []string {
	var out []string
	for _, kv := range environ {
		name := kv
		if i := strings.IndexByte(kv, '='); i > 0 {
			name = kv[:i]
		}
		if keystore.IsPrivateKeyEnv(name) {
			continue
		}
		out = append(out, kv)
	}
	return out
}

// Secrets returns the byte strings to redact: when redactValues is set, every
// string value of length >= MinRedactLen and every value at a path the schema
// marks x-zeze-secret (all in their encoded variants); always every private
// key in all its encodings.
func Secrets(d *doc.Value, s *schema.Schema, keys []crypt.PrivateKey, redactValues bool) [][]byte {
	set := map[string]bool{}
	add := func(x string) {
		if x == "" {
			return
		}
		for _, v := range redact.Variants(x) {
			if v != "" {
				set[v] = true
			}
		}
	}
	if redactValues && d != nil {
		for _, l := range doc.Leaves(d) {
			secret := s != nil && s.Secret(l.Pointer)
			if l.Value.Kind == doc.String {
				if len(l.Value.S) >= MinRedactLen || secret {
					add(l.Value.S)
				}
				if secret {
					add(string(doc.Canonical(l.Value)))
				}
				continue
			}
			if secret {
				add(string(doc.Canonical(l.Value)))
			}
		}
	}
	for _, k := range keys {
		enc := k.String()
		add(enc)
		add(strings.TrimPrefix(enc, crypt.PrivateKeyPrefix))
		add(base64.StdEncoding.EncodeToString(k[:]))
		add(hex.EncodeToString(k[:]))
		// Direct encodings of the raw key bytes (Variants encodes strings,
		// not these bytes).
		for _, x := range []string{
			base64.StdEncoding.EncodeToString(k[:]),
			base64.RawStdEncoding.EncodeToString(k[:]),
			base64.URLEncoding.EncodeToString(k[:]),
			base64.RawURLEncoding.EncodeToString(k[:]),
			hex.EncodeToString(k[:]),
			strings.ToUpper(hex.EncodeToString(k[:])),
		} {
			set[x] = true
		}
	}
	out := make([][]byte, 0, len(set))
	for x := range set {
		out = append(out, []byte(x))
	}
	sort.Slice(out, func(i, j int) bool { return string(out[i]) < string(out[j]) })
	return out
}

func validEnvAssignmentName(name string) bool { return validEnvName(name) }

// BuildEnv computes the child environment (without file-format plumbing) and
// returns it with the names zeze injected. docFile is the value for the
// file format's variable.
func BuildEnv(o Options, docFile string) ([]string, error) {
	environ := o.Environ
	if environ == nil {
		environ = os.Environ()
	}
	env := newEnvList(ChildEnv(environ))
	format := o.Format
	if format == "" {
		format = FormatJSON
	}
	envName := o.EnvName
	if envName == "" {
		envName = DefaultEnvName
	}
	if !validEnvName(envName) {
		return nil, runErr(xerr.KindUsage, "--env-name must be a valid environment variable name")
	}
	if keystore.IsPrivateKeyEnv(envName) {
		return nil, runErr(xerr.KindUsage, "--env-name may not name a private-key variable")
	}
	if format != FormatFlatten && config.DeniedRunEnvName(envName) {
		return nil, runErr(xerr.KindUsage, "--env-name (or run.envName) %s is denylisted: it loads or runs code, or belongs to zeze", envName)
	}
	if o.Doc == nil || o.Doc.Kind != doc.Object {
		return nil, runErr(xerr.KindValidation, "the document must be an object")
	}
	inject := func(name, val string) {
		if env.has(name) && !o.Override {
			return
		}
		env.set(name, val)
	}
	// The document variable always carries this run's document: an inherited
	// value (from the shell or an outer zeze run) must not win silently.
	switch format {
	case FormatJSON:
		env.set(envName, string(doc.MarshalCompact(o.Doc)))
	case FormatFlatten:
		vars, err := FlattenExcept(o.Doc, o.Prefix, mappedPointers(o.Env))
		if err != nil {
			return nil, asRun(err)
		}
		names := make([]string, 0, len(vars))
		for n := range vars {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			inject(n, vars[n])
		}
	case FormatFile:
		env.set(envName, docFile)
	default:
		return nil, runErr(xerr.KindUsage, "--format must be json, flatten or file")
	}
	for _, spec := range o.Env {
		i := strings.IndexByte(spec, '=')
		if i <= 0 {
			return nil, runErr(xerr.KindUsage, "--env expects NAME=/json/pointer")
		}
		name, ptr := spec[:i], spec[i+1:]
		if !validEnvAssignmentName(name) {
			return nil, runErr(xerr.KindUsage, "--env %s: invalid variable name", name)
		}
		if keystore.IsPrivateKeyEnv(name) {
			return nil, runErr(xerr.KindUsage, "--env %s: private-key variables cannot be injected", name)
		}
		toks, err := doc.ParsePointer(ptr)
		if err != nil {
			return nil, runErr(xerr.KindUsage, "--env %s: invalid JSON Pointer", name)
		}
		v, err := doc.Lookup(o.Doc, toks)
		if err != nil {
			return nil, runErr(xerr.KindValidation, "--env %s: %s not found", name, displayPointer(ptr))
		}
		val, ok := scalarEnv(v)
		if !ok {
			continue
		}
		if strings.IndexByte(val, 0) >= 0 {
			return nil, runErr(xerr.KindValidation, "--env %s: value contains NUL", name)
		}
		if env.has(name) && !o.Override {
			continue
		}
		env.set(name, val)
	}
	return env.environ(), nil
}

// mappedPointers returns the escaped pointers named by --env specs.
func mappedPointers(specs []string) map[string]bool {
	out := map[string]bool{}
	for _, spec := range specs {
		i := strings.IndexByte(spec, '=')
		if i <= 0 {
			continue
		}
		toks, err := doc.ParsePointer(spec[i+1:])
		if err != nil {
			continue
		}
		out[doc.FormatPointer(toks)] = true
	}
	return out
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Run builds the environment (strip ZEZE_PRIVATE_KEY*, inject per Format),
// starts the child, redacts output, and returns the child's exit code.
// zeze-side failures return (125, err).
func Run(ctx context.Context, o Options) (int, error) {
	if len(o.Args) == 0 {
		return xerr.CodeRun, runErr(xerr.KindUsage, "missing command (use zeze run -- <command> [args...])")
	}
	if o.Stdout == nil {
		o.Stdout = os.Stdout
	}
	if o.Stderr == nil {
		o.Stderr = os.Stderr
	}
	if o.Stdin == nil {
		o.Stdin = os.Stdin
	}
	SweepStale(o.Home)

	// Instance registry entry (also holds the file-format document).
	var instDir string
	if o.Home != "" {
		instDir = filepath.Join(o.Home, "run", newID())
		if err := os.MkdirAll(instDir, 0o700); err != nil {
			return xerr.CodeRun, runErr(xerr.KindFailure, "cannot create the run directory")
		}
		_ = os.Chmod(filepath.Join(o.Home, "run"), 0o700)
		if err := os.WriteFile(filepath.Join(instDir, ownerFile), []byte(strconv.Itoa(os.Getpid())+"\n"), 0o600); err != nil {
			os.RemoveAll(instDir)
			return xerr.CodeRun, runErr(xerr.KindFailure, "cannot register the run instance")
		}
		defer os.RemoveAll(instDir)
	}

	var extra []*os.File
	docFile := ""
	if o.Format == FormatFile {
		data := doc.Marshal(o.Doc)
		f, err := memfdDocument(data)
		clear(data)
		if err == nil && f != nil {
			extra = append(extra, f)
			defer f.Close()
			docFile = "/dev/fd/3"
		} else {
			if instDir == "" {
				return xerr.CodeRun, runErr(xerr.KindFailure, "--format file needs the zeze home directory")
			}
			p := filepath.Join(instDir, "document.json")
			data := doc.Marshal(o.Doc)
			werr := os.WriteFile(p, data, 0o600)
			clear(data)
			if werr != nil {
				return xerr.CodeRun, runErr(xerr.KindFailure, "cannot write the document file")
			}
			docFile = p
		}
	}

	env, err := BuildEnv(o, docFile)
	if err != nil {
		return xerr.CodeRun, err
	}

	secrets := Secrets(o.Doc, o.Schema, o.PrivateKeys, o.Redact)
	stdout := redact.NewWriter(o.Stdout, secrets)
	stderr := redact.NewWriter(o.Stderr, secrets)

	path, err := exec.LookPath(o.Args[0])
	if err != nil {
		return xerr.CodeRun, runErr(xerr.KindExec, "command not found: %s", o.Args[0])
	}
	cmd := exec.Command(path, o.Args[1:]...)
	cmd.Args[0] = o.Args[0]
	cmd.Env = env
	cmd.Stdin = o.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.ExtraFiles = extra

	sigs := make(chan os.Signal, 8)
	signal.Notify(sigs, forwardedSignals...)
	defer signal.Stop(sigs)

	if err := cmd.Start(); err != nil {
		return xerr.CodeRun, runErr(xerr.KindExec, "cannot start %s", o.Args[0])
	}
	done := make(chan struct{})
	go func() {
		for {
			select {
			case s := <-sigs:
				_ = cmd.Process.Signal(s)
			case <-ctx.Done():
				_ = cmd.Process.Signal(os.Interrupt)
				<-done
				return
			case <-done:
				return
			}
		}
	}()
	werr := cmd.Wait()
	close(done)
	_ = stdout.Close()
	_ = stderr.Close()
	if werr != nil {
		var ee *exec.ExitError
		if !errors.As(werr, &ee) {
			return xerr.CodeRun, runErr(xerr.KindExec, "waiting for %s failed", o.Args[0])
		}
	}
	return exitCode(cmd.ProcessState), nil
}

const ownerFile = "owner"

// Instance is one registered run process.
type Instance struct {
	ID  string
	PID int
}

// Instances lists registered run instances whose owner process is alive.
func Instances(home string) []Instance {
	var out []Instance
	if home == "" {
		return nil
	}
	ents, err := os.ReadDir(filepath.Join(home, "run"))
	if err != nil {
		return nil
	}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		pid := readOwner(filepath.Join(home, "run", e.Name()))
		if pid > 0 && processAlive(pid) {
			out = append(out, Instance{ID: e.Name(), PID: pid})
		}
	}
	return out
}

func readOwner(dir string) int {
	b, err := os.ReadFile(filepath.Join(dir, ownerFile))
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0
	}
	return pid
}

// SweepStale removes run/<id>/ entries whose owner process is gone.
func SweepStale(home string) {
	if home == "" {
		return
	}
	dir := filepath.Join(home, "run")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		p := filepath.Join(dir, e.Name())
		if !e.IsDir() {
			continue
		}
		pid := readOwner(p)
		if pid == os.Getpid() {
			continue
		}
		if pid <= 0 {
			// Possibly being created right now; only remove when old.
			if st, err := os.Stat(p); err != nil || time.Since(st.ModTime()) < time.Minute {
				continue
			}
		} else if processAlive(pid) {
			continue
		}
		os.RemoveAll(p)
	}
}

func processAlive(pid int) bool { return paths.ProcessAlive(pid) }
