package runner

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/CGuiho/zeze/internal/crypt"
	"github.com/CGuiho/zeze/internal/doc"
	"github.com/CGuiho/zeze/internal/jsonc"
	"github.com/CGuiho/zeze/internal/schema"
	"github.com/CGuiho/zeze/internal/xerr"
)

const sentinel = "SENTINEL-runner-42"

func parse(t *testing.T, s string) *doc.Value {
	t.Helper()
	v, err := jsonc.Parse([]byte(s), true)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestEnvName(t *testing.T) {
	cases := []struct{ ptr, prefix, want string }{
		{"/database/url", "", "DATABASE_URL"},
		{"/features/0", "", "FEATURES_0"},
		{"/apiKey", "", "APIKEY"},
		{"/a~1b/c~0d", "", "A_B_C_D"},
		{"/0abc", "", "_0ABC"},
		{"/db-host.name", "APP_", "APP_DB_HOST_NAME"},
		{"/é", "", "_"},
	}
	for _, c := range cases {
		got, err := EnvName(c.ptr, c.prefix)
		if err != nil || got != c.want {
			t.Errorf("%s: got %q %v want %q", c.ptr, got, err, c.want)
		}
	}
}

func TestFlattenRules(t *testing.T) {
	v := parse(t, `{"database":{"url":"u","pool":10,"ssl":true,"none":null},"tags":[],"o":{},"n":1.50}`)
	got, err := Flatten(v, "")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"DATABASE_URL": "u", "DATABASE_POOL": "10", "DATABASE_SSL": "true", "N": "1.50"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for k, w := range want {
		if got[k] != w {
			t.Errorf("%s = %q want %q", k, got[k], w)
		}
	}
	for _, bad := range []string{`{"a_b":1,"a":{"b":2}}`, `{"LD_PRELOAD":"x"}`, `{"node_options":"x"}`, `{"zeze":{"x":1}}`, `{"env":"x"}`} {
		if _, err := Flatten(parse(t, bad), ""); err == nil {
			t.Errorf("%s: expected an error", bad)
		} else if strings.Contains(err.Error(), "\"x\"") {
			t.Errorf("error leaked a value: %v", err)
		}
	}
	if _, err := Flatten(parse(t, `{"x":1}`), "1BAD"); xerr.CodeOf(err) != xerr.CodeUsage {
		t.Errorf("bad prefix must be a usage error: %v", err)
	}
}

func TestBuildEnv(t *testing.T) {
	d := parse(t, `{"db":{"url":"postgres://x","port":5432},"flag":null}`)
	environ := []string{"PATH=/bin", "ZEZE_PRIVATE_KEY=zeze-sk-1:abc", "ZEZE_PRIVATE_KEY_K_00=zz", "DB_URL=keep", "HOME=/h"}
	get := func(env []string, name string) (string, bool) {
		for _, kv := range env {
			if strings.HasPrefix(kv, name+"=") {
				return kv[len(name)+1:], true
			}
		}
		return "", false
	}
	env, err := BuildEnv(Options{Doc: d, Environ: environ}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := get(env, "ZEZE_PRIVATE_KEY"); ok {
		t.Fatal("private key not stripped")
	}
	if _, ok := get(env, "ZEZE_PRIVATE_KEY_K_00"); ok {
		t.Fatal("dedicated private key not stripped")
	}
	if v, _ := get(env, "ZEZE_CONFIG"); v != `{"db":{"url":"postgres://x","port":5432},"flag":null}` {
		t.Fatalf("ZEZE_CONFIG = %q", v)
	}
	env, _ = BuildEnv(Options{Doc: d, Environ: environ, Format: FormatFlatten, Env: []string{"PORT=/db/port", "WHOLE=/db"}}, "")
	if v, _ := get(env, "DB_URL"); v != "keep" {
		t.Fatal("existing variables must win without --override")
	}
	if v, _ := get(env, "PORT"); v != "5432" {
		t.Fatal("--env not applied")
	}
	if v, _ := get(env, "WHOLE"); v != `{"url":"postgres://x","port":5432}` {
		t.Fatalf("--env container = %q", v)
	}
	if _, ok := get(env, "FLAG"); ok {
		t.Fatal("null must not be set")
	}
	env, _ = BuildEnv(Options{Doc: d, Environ: environ, Format: FormatFlatten, Override: true}, "")
	if v, _ := get(env, "DB_URL"); v != "postgres://x" {
		t.Fatal("--override must replace")
	}
	env, _ = BuildEnv(Options{Doc: d, Environ: environ, Format: FormatJSON, EnvName: "CFG"}, "")
	if _, ok := get(env, "CFG"); !ok {
		t.Fatal("--env-name ignored")
	}
	for _, o := range []Options{
		{Doc: d, Environ: environ, Env: []string{"ZEZE_PRIVATE_KEY=/db/url"}},
		{Doc: d, Environ: environ, Env: []string{"=nope"}},
		{Doc: d, Environ: environ, Env: []string{"X=/missing"}},
		{Doc: d, Environ: environ, Format: "yaml"},
		{Doc: d, Environ: environ, EnvName: "1X"},
	} {
		if _, err := BuildEnv(o, ""); xerr.CodeOf(err) != xerr.CodeRun {
			t.Errorf("%+v: want exit 125, got %v", o.Env, err)
		}
	}
}

func TestSecrets(t *testing.T) {
	d := parse(t, `{"long":"`+sentinel+`","short":"abc","pin":1234,"flag":true}`)
	sch, err := schema.Compile(parse(t, `{"type":"object","properties":{"pin":{"x-zeze-secret":true}}}`), "t")
	if err != nil {
		t.Fatal(err)
	}
	sk, _ := crypt.GenerateKey()
	set := map[string]bool{}
	for _, s := range Secrets(d, sch, []crypt.PrivateKey{sk}, true) {
		set[string(s)] = true
	}
	for _, want := range []string{sentinel, base64.StdEncoding.EncodeToString([]byte(sentinel)), "1234", sk.String()} {
		if !set[want] {
			t.Errorf("missing %q", want)
		}
	}
	if set["abc"] || set["true"] {
		t.Error("short non-secret values must not be redacted")
	}
	set = map[string]bool{}
	for _, s := range Secrets(d, sch, []crypt.PrivateKey{sk}, false) {
		set[string(s)] = true
	}
	if set[sentinel] || !set[sk.String()] {
		t.Error("bypass disables value redaction but never key redaction")
	}
}

func TestRunRedactsAndPassesExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	sk, _ := crypt.GenerateKey()
	home := t.TempDir()
	d := parse(t, `{"url":"`+sentinel+`","n":3}`)
	var out, errb bytes.Buffer
	t.Setenv("ZEZE_PRIVATE_KEY", sk.String())
	code, err := Run(context.Background(), Options{
		Doc: d, Redact: true, PrivateKeys: []crypt.PrivateKey{sk}, Home: home,
		Args:  []string{"sh", "-c", `printf '%s\n' "$ZEZE_CONFIG"; printf 'key=%s\n' "${ZEZE_PRIVATE_KEY:-none}"; printf '%s' "$ZEZE_CONFIG" | base64 >&2; exit 3`},
		Stdin: strings.NewReader(""), Stdout: &out, Stderr: &errb,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 3 {
		t.Fatalf("exit code %d", code)
	}
	if strings.Contains(out.String(), sentinel) {
		t.Fatalf("value leaked: %s", out.String())
	}
	if !strings.Contains(out.String(), `"url":"[REDACTED]"`) || !strings.Contains(out.String(), "key=none") {
		t.Fatalf("unexpected output: %s", out.String())
	}
	if ents, _ := os.ReadDir(filepath.Join(home, "run")); len(ents) != 0 {
		t.Fatal("run registry entry not cleaned up")
	}
}

func TestRunFileFormat(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	home := t.TempDir()
	d := parse(t, `{"a":"`+sentinel+`"}`)
	var out bytes.Buffer
	code, err := Run(context.Background(), Options{
		Doc: d, Format: FormatFile, Redact: true, Home: home,
		Args:  []string{"sh", "-c", `test -r "$ZEZE_CONFIG" && grep -c a "$ZEZE_CONFIG"`},
		Stdin: strings.NewReader(""), Stdout: &out, Stderr: &out,
	})
	if err != nil || code != 0 {
		t.Fatalf("code %d err %v out %s", code, err, out.String())
	}
	if ents, _ := os.ReadDir(filepath.Join(home, "run")); len(ents) != 0 {
		t.Fatal("document file not cleaned up")
	}
}

func TestRunMissingCommand(t *testing.T) {
	code, err := Run(context.Background(), Options{Doc: parse(t, `{}`), Args: []string{"definitely-not-a-command-xyz"}, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	if code != 125 || xerr.KindOf(err) != xerr.KindExec {
		t.Fatalf("got %d %v", code, err)
	}
}

func TestSweepStale(t *testing.T) {
	home := t.TempDir()
	dead := filepath.Join(home, "run", "dead")
	os.MkdirAll(dead, 0o700)
	os.WriteFile(filepath.Join(dead, ownerFile), []byte("999999999\n"), 0o600)
	live := filepath.Join(home, "run", "live")
	os.MkdirAll(live, 0o700)
	os.WriteFile(filepath.Join(live, ownerFile), []byte(strconv.Itoa(os.Getppid())+"\n"), 0o600)
	SweepStale(home)
	if _, err := os.Stat(dead); err == nil {
		t.Fatal("stale entry kept")
	}
	if _, err := os.Stat(live); err != nil {
		t.Fatal("live entry removed")
	}
	if len(Instances(home)) != 1 {
		t.Fatal("live instance not listed")
	}
}

func TestFlattenDeniedPathMappedExplicitly(t *testing.T) {
	d := parse(t, `{"ld":{"preload":"x"},"db":{"url":"u"}}`)
	if _, err := Flatten(d, ""); err == nil {
		t.Fatal("a denylisted name must fail without an explicit mapping")
	}
	env, err := BuildEnv(Options{Doc: d, Format: FormatFlatten, Env: []string{"LD_PRELOAD=/ld/preload"}, Environ: []string{}}, "")
	if err != nil {
		t.Fatalf("explicit --env mapping must allow the denylisted path: %v", err)
	}
	got := strings.Join(env, "\n")
	if !strings.Contains(got, "LD_PRELOAD=x") || !strings.Contains(got, "DB_URL=u") {
		t.Fatalf("env: %v", env)
	}
	// A mapping of another path does not exempt the denylisted one.
	if _, err := BuildEnv(Options{Doc: d, Format: FormatFlatten, Env: []string{"X=/db/url"}, Environ: []string{}}, ""); err == nil {
		t.Fatal("only the mapped path is exempt")
	}
}

func TestBuildEnvRejectsDenylistedEnvName(t *testing.T) {
	d := parse(t, `{"a":"b"}`)
	for _, format := range []string{FormatJSON, FormatFile} {
		for _, name := range []string{"BASH_ENV", "ENV", "PAGER", "EDITOR", "GIT_SSH_COMMAND", "PROMPT_COMMAND", "PS4", "LD_PRELOAD", "ZEZE_USER_HOME", "ZEZE_", "bash_env"} {
			if _, err := BuildEnv(Options{Doc: d, Format: format, EnvName: name, Environ: []string{}}, "/dev/fd/3"); xerr.CodeOf(err) != xerr.CodeRun {
				t.Errorf("%s %s: want exit 125, got %v", format, name, err)
			}
		}
	}
	if _, err := BuildEnv(Options{Doc: d, EnvName: DefaultEnvName, Environ: []string{}}, ""); err != nil {
		t.Fatalf("default name refused: %v", err)
	}
}

// The document lands in ZEZE_CONFIG by default; the abandoned ZEZE_
// name is an ordinary denylisted ZEZE_* name.
func TestBuildEnvDefaultEnvName(t *testing.T) {
	d := parse(t, `{"a":"b"}`)
	if DefaultEnvName != "ZEZE_CONFIG" {
		t.Fatalf("DefaultEnvName = %q", DefaultEnvName)
	}
	env, err := BuildEnv(Options{Doc: d, Environ: []string{}}, "")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, `ZEZE_CONFIG={"a":"b"}`) {
		t.Fatalf("default injection not in ZEZE_CONFIG: %v", env)
	}
	if strings.Contains(joined, "ZEZE_={\"a\":\"b\"}") {
		t.Fatalf("abandoned name still injected: %v", env)
	}
	// The default name is accepted explicitly: it is the denylist exception.
	if _, err := BuildEnv(Options{Doc: d, Format: FormatJSON, EnvName: "ZEZE_CONFIG", Environ: []string{}}, ""); err != nil {
		t.Fatalf("default name must be accepted as --env-name: %v", err)
	}
	// The abandoned ZEZE_ name is refused as denylisted.
	if _, err := BuildEnv(Options{Doc: d, Format: FormatJSON, EnvName: "ZEZE_", Environ: []string{}}, ""); xerr.CodeOf(err) != xerr.CodeRun {
		t.Fatalf("ZEZE_ must be refused as denylisted: %v", err)
	}
}

func TestFlattenDenylistCoversCodeExecutingNames(t *testing.T) {
	for _, name := range []string{"GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0", "GIT_SSH_COMMAND", "GIT_EXTERNAL_DIFF", "GIT_PAGER", "PAGER", "EDITOR", "VISUAL", "JAVA_TOOL_OPTIONS", "_JAVA_OPTIONS", "PROMPT_COMMAND", "PS4", "SHELLOPTS", "BASHOPTS", "PYTHONHOME", "RUBYLIB", "NODE_PATH", "PATH"} {
		if !Denied(name) {
			t.Errorf("%s is not denylisted", name)
		}
	}
	d := parse(t, `{"git_config_count":"1","git_config_key_0":"core.fsmonitor"}`)
	if _, err := Flatten(d, ""); err == nil {
		t.Fatal("flatten produced GIT_CONFIG_* variables")
	}
}

func TestBuildEnvDocumentVariableAlwaysSet(t *testing.T) {
	d := parse(t, `{"a":"b"}`)
	for _, format := range []string{FormatJSON, FormatFile} {
		env, err := BuildEnv(Options{Doc: d, Format: format, Environ: []string{"ZEZE_CONFIG={\"stale\":1}"}}, "/dev/fd/3")
		if err != nil {
			t.Fatal(err)
		}
		for _, kv := range env {
			if strings.Contains(kv, "stale") {
				t.Fatalf("%s: inherited ZEZE_CONFIG won: %v", format, env)
			}
		}
	}
}

func TestSecretsCoverRawKeyEncodings(t *testing.T) {
	sk, _ := crypt.GenerateKey()
	set := map[string]bool{}
	for _, s := range Secrets(nil, nil, []crypt.PrivateKey{sk}, false) {
		set[string(s)] = true
	}
	for _, want := range []string{
		base64.RawStdEncoding.EncodeToString(sk[:]),
		base64.StdEncoding.EncodeToString(sk[:]),
		base64.URLEncoding.EncodeToString(sk[:]),
		base64.RawURLEncoding.EncodeToString(sk[:]),
		strings.ToUpper(hex.EncodeToString(sk[:])),
		hex.EncodeToString(sk[:]),
	} {
		if !set[want] {
			t.Errorf("raw key encoding %q is not redacted", want)
		}
	}
}
