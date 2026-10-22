package e2e

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunDefaultJSON(t *testing.T) {
	needSh(t)
	p := newProject(t)
	p.setup(`{"database":{"url":"postgres://u:pw@h/db","pool":20},"apiKey":"key-run-json","debug":false,"short":"abc"}`)

	// Without the bypass the child gets the real document but its output is redacted.
	r := p.expect(0, "run", "--", "sh", "-c", `printf '%s' "$ZEZE_CONFIG"`)
	if r.stdout != `{"database":{"url":"[REDACTED]","pool":20},"apiKey":"[REDACTED]","debug":false,"short":"abc"}` {
		t.Fatalf("redacted ZEZE_CONFIG: %s", r.stdout)
	}
	// The child really received the values (checked inside the child).
	p.expect(0, "run", "--", "sh", "-c", `case "$ZEZE_CONFIG" in *'"apiKey":"key-run-json"'*) exit 0;; *) exit 1;; esac`)
	// The exact document reaches the child.
	if got := p.revealConfig(); got != `{"database":{"url":"postgres://u:pw@h/db","pool":20},"apiKey":"key-run-json","debug":false,"short":"abc"}` {
		t.Fatalf("ZEZE_CONFIG = %s", got)
	}
	// --env-name renames the variable; an existing variable wins unless --override.
	r = p.expect(0, "run", "--env-name", "APP_CFG", "--", "sh", "-c", `printf '%s|%s' "${APP_CFG:+set}" "${ZEZE_CONFIG:-unset}"`)
	if r.stdout != "set|unset" {
		t.Fatalf("--env-name: %q", r.stdout)
	}
	// The default name is the denylist exception; the abandoned ZEZE_ name
	// is refused like any other ZEZE_* name.
	r = p.expect(0, "run", "--env-name", "ZEZE_CONFIG", "--", "sh", "-c", `printf '%.1s' "$ZEZE_CONFIG"`)
	if r.stdout != "{" {
		t.Fatalf("--env-name ZEZE_CONFIG: %q", r.stdout)
	}
	r = p.run("run", "--env-name", "ZEZE_", "--", "true")
	if r.code != 125 || !strings.HasPrefix(r.stderr, "[ZEZE:USAGE]") {
		t.Fatalf("--env-name ZEZE_ must be refused as denylisted\n%s", r)
	}
	// The document variable always carries this run's document: an inherited
	// ZEZE_CONFIG (shell, outer run) never wins silently.
	r = p.expectEnv(0, []string{"ZEZE_CONFIG=preexisting"}, "run", "--", "sh", "-c", `printf '%.1s' "$ZEZE_CONFIG"`)
	if r.stdout != "{" {
		t.Fatalf("an inherited ZEZE_CONFIG must be replaced: %q", r.stdout)
	}
	r = p.expectEnv(0, []string{"ZEZE_CONFIG=preexisting"}, "run", "--override", "--", "sh", "-c", `printf '%.1s' "$ZEZE_CONFIG"`)
	if r.stdout != "{" {
		t.Fatalf("--override: %q", r.stdout)
	}
	// run.envName and run.format from zeze.yaml.
	p.write("zeze.yaml", "pairs:\n  - name: config\nrun:\n  envName: CFG_FROM_YAML\n")
	r = p.expect(0, "run", "--", "sh", "-c", `printf '%s' "${CFG_FROM_YAML:+yes}"`)
	if r.stdout != "yes" {
		t.Fatalf("run.envName ignored: %q", r.stdout)
	}
	// A denylisted run.envName in zeze.yaml is refused when the file loads.
	p.write("zeze.yaml", "pairs:\n  - name: config\nrun:\n  envName: ZEZE_\n")
	r = p.run("run", "--", "true")
	if r.code == 0 || !strings.Contains(r.stderr, "/run/envName") {
		t.Fatalf("run.envName ZEZE_ must be refused\n%s", r)
	}
}

func TestRunFlatten(t *testing.T) {
	needSh(t)
	p := newProject(t)
	p.setup(`{"database":{"url":"flat-url-value","pool":20},"features":[1.50,"x"],"apiKey":"flat-api-key","a~b":true,"9lives":"cat","nothing":null,"empty":{}}`)

	r := p.expect(0, "run", "--dangerously-bypass-protections", "--format", "flatten", "--", "sh", "-c",
		`printf '%s|%s|%s|%s|%s|%s|%s|%s' "$DATABASE_URL" "$DATABASE_POOL" "$FEATURES_0" "$FEATURES_1" "$APIKEY" "$A_B" "$_9LIVES" "${NOTHING-unset}"`)
	if r.stdout != "flat-url-value|20|1.50|x|flat-api-key|true|cat|unset" {
		t.Fatalf("flatten values: %q", r.stdout)
	}
	r = p.expect(0, "run", "--dangerously-bypass-protections", "--format", "flatten", "--prefix", "APP_", "--", "sh", "-c", `printf '%s|%s' "$APP_DATABASE_URL" "${DATABASE_URL-unset}"`)
	if r.stdout != "flat-url-value|unset" {
		t.Fatalf("--prefix: %q", r.stdout)
	}
	// Redacted without the bypass.
	r = p.expect(0, "run", "--format", "flatten", "--", "sh", "-c", `printf '%s %s' "$DATABASE_URL" "$DATABASE_POOL"`)
	if r.stdout != "[REDACTED] 20" {
		t.Fatalf("flatten redaction: %q", r.stdout)
	}

	// Collision after normalisation: exit 125, the child never starts.
	p.expect(0, "set", "/database_url", "collides")
	r = p.run("run", "--format", "flatten", "--", "sh", "-c", "echo started")
	if r.code != 125 || !strings.HasPrefix(r.stderr, "[ZEZE:") || strings.Contains(r.stdout, "started") || !strings.Contains(r.stderr, "DATABASE_URL") {
		t.Fatalf("collision: want 125\n%s", r)
	}
	assertNo(t, "collision error", r.out(), "flat-url-value", "collides")
	p.expect(0, "unset", "/database_url")

	// Denylisted names are refused in flatten mode.
	for _, ptr := range []string{"/ld/preload", "/node_options", "/zeze/private_key", "/pythonpath", "/bash_env"} {
		p.expect(0, "set", ptr, "evil-value")
		r = p.run("run", "--format", "flatten", "--", "sh", "-c", "echo started")
		if r.code != 125 || strings.Contains(r.stdout, "started") {
			t.Fatalf("%s: flatten must refuse a denylisted name with 125\n%s", ptr, r)
		}
		assertNo(t, "denylist error", r.out(), "evil-value")
		// json mode is unaffected.
		p.expect(0, "run", "--", "true")
		p.expect(0, "unset", ptr)
	}
	// A denylisted path may still be mapped explicitly with --env.
	p.expect(0, "set", "/ld/library_path", "/opt/lib")
	r = p.run("run", "--dangerously-bypass-protections", "--format", "flatten", "--env", "LD_LIBRARY_PATH=/ld/library_path", "--", "sh", "-c", `printf '%s|%s' "$LD_LIBRARY_PATH" "$DATABASE_URL"`)
	if r.code != 0 || r.stdout != "/opt/lib|flat-url-value" {
		t.Fatalf("explicit --env mapping of a denylisted path must be allowed\n%s", r)
	}
	// check and list report flatten problems in advance.
	r = p.run("list")
	if !strings.Contains(r.stderr, "LD_LIBRARY_PATH") {
		t.Fatalf("list must warn about the denylisted name\n%s", r)
	}
}

func TestRunFileFormat(t *testing.T) {
	needSh(t)
	p := newProject(t)
	p.setup(`{"svc":{"token":"file-format-token"},"n":1.0}`)
	r := p.expect(0, "run", "--dangerously-bypass-protections", "--format", "file", "--", "sh", "-c", `cat "$ZEZE_CONFIG"`)
	if stripWS(r.stdout) != `{"svc":{"token":"file-format-token"},"n":1.0}` {
		t.Fatalf("--format file document: %q", r.stdout)
	}
	r = p.expect(0, "run", "--format", "file", "--", "sh", "-c", `printf '%s' "$ZEZE_CONFIG"`)
	path := r.stdout
	if path == "" {
		t.Fatal("--format file: no path")
	}
	// Nothing is left behind on disk.
	if !strings.HasPrefix(path, "/dev/fd/") {
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("document file %s survived the child", path)
		}
	}
	if ents, _ := os.ReadDir(filepath.Join(p.jhome(), "run")); len(ents) != 0 {
		t.Fatalf("run registry not cleaned: %v", ents)
	}
	r = p.expect(0, "run", "--format", "file", "--", "sh", "-c", `cat "$ZEZE_CONFIG"`)
	if strings.Contains(r.stdout, "file-format-token") || !strings.Contains(r.stdout, "[REDACTED]") {
		t.Fatalf("file-format output not redacted: %s", r.stdout)
	}
}

func TestRunEnvExtras(t *testing.T) {
	needSh(t)
	p := newProject(t)
	p.setup(`{"db":{"url":"extra-db-url","port":5432,"opt":null},"list":["a","b"]}`)
	r := p.expect(0, "run", "--dangerously-bypass-protections",
		"--env", "DB=/db/url", "--env", "PORT=/db/port", "--env", "LIST=/list", "--env", "OPT=/db/opt",
		"--", "sh", "-c", `printf '%s|%s|%s|%s|%s' "$DB" "$PORT" "$LIST" "${OPT-unset}" "${ZEZE_CONFIG:+json}"`)
	if r.stdout != `extra-db-url|5432|["a","b"]|unset|json` {
		t.Fatalf("--env extras: %q\n%s", r.stdout, r)
	}
	r = p.run("run", "--env", "X=/missing/pointer", "--", "true")
	if r.code != 125 || !strings.HasPrefix(r.stderr, "[ZEZE:VALIDATION]") {
		t.Fatalf("missing --env pointer: want 125 [ZEZE:VALIDATION]\n%s", r)
	}
	r = p.run("run", "--env", "noequals", "--", "true")
	if r.code != 125 || !strings.HasPrefix(r.stderr, "[ZEZE:") {
		t.Fatalf("bad --env: want 125\n%s", r)
	}
	r = p.run("run", "--env", "ZEZE_PRIVATE_KEY=/db/url", "--", "true")
	if r.code != 125 {
		t.Fatalf("--env must not inject a private-key variable\n%s", r)
	}
}

func TestRunExitCodesAndFailures(t *testing.T) {
	needSh(t)
	p := newProject(t)
	p.setup(`{"a":"exit-code-value"}`)
	for _, c := range []string{"0", "1", "7", "42", "124"} {
		r := p.run("run", "--", "sh", "-c", "exit "+c)
		if want := c; r.stderr != "" || itoa(r.code) != want {
			t.Fatalf("child exit %s passed through as %d\n%s", c, r.code, r)
		}
	}
	r := p.run("run", "--", "definitely-not-a-command-e2e")
	if r.code != 125 || !strings.HasPrefix(r.stderr, "[ZEZE:") {
		t.Fatalf("missing command: want 125\n%s", r)
	}
	r = p.run("run", "--format", "yaml", "--", "true")
	if r.code != 125 || !strings.HasPrefix(r.stderr, "[ZEZE:USAGE]") {
		t.Fatalf("bad --format: want 125 [ZEZE:USAGE]\n%s", r)
	}
	restore := p.hideKeys()
	r = p.run("run", "--", "sh", "-c", "echo started")
	if r.code != 125 || !strings.HasPrefix(r.stderr, "[ZEZE:KEY]") || strings.Contains(r.stdout, "started") {
		t.Fatalf("no key: want 125 [ZEZE:KEY]\n%s", r)
	}
	restore()
	// A document that fails the schema never reaches the child.
	p.write("config.schema.json", `{"type":"object","properties":{"a":{"type":"number"}}}`)
	r = p.run("run", "--", "sh", "-c", "echo started")
	if r.code != 125 || !strings.HasPrefix(r.stderr, "[ZEZE:VALIDATION]") || strings.Contains(r.stdout, "started") {
		t.Fatalf("invalid document: want 125 [ZEZE:VALIDATION]\n%s", r)
	}
	assertNo(t, "validation failure", r.out(), "exit-code-value")
	// stdin is inherited.
	p.write("config.schema.json", `{"type":"object"}`)
	r = p.runStdin("piped-input\n", "run", "--", "sh", "-c", "read x; printf '%s' \"$x\"")
	if r.code != 0 || r.stdout != "piped-input" {
		t.Fatalf("stdin not inherited\n%s", r)
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestRunStripsPrivateKeys(t *testing.T) {
	needSh(t)
	p := newProject(t)
	p.setup(`{"a":"strip-value"}`)
	keyID, sk := p.privateKey()
	envName := "ZEZE_PRIVATE_KEY_" + strings.ToUpper(keyID)
	// The key comes from the environment only (agent / CI machine).
	restore := p.hideKeys()
	env := []string{envName + "=" + sk, "ZEZE_PRIVATE_KEY=" + sk, "ZEZE_PRIVATE_KEY_OTHER=x"}
	r := p.expectEnv(0, env, "run", "--", "sh", "-c", `env | grep -c '^ZEZE_PRIVATE_KEY' ; printf '%s' "${ZEZE_PRIVATE_KEY-unset}"`)
	if r.stdout != "0\nunset" {
		t.Fatalf("private-key variables reached the child: %q", r.stdout)
	}
	// Even with the bypass, private keys stay out of the child and redacted.
	r = p.expectEnv(0, env, "run", "--dangerously-bypass-protections", "--", "sh", "-c", `env | grep -c '^ZEZE_PRIVATE_KEY'; printf '%s\n' "$ZEZE_CONFIG"`)
	if !strings.HasPrefix(r.stdout, "0\n") || !strings.Contains(r.stdout, "strip-value") {
		t.Fatalf("bypass run: %q", r.stdout)
	}
	restore()
	keyFile := p.keyFiles()[0]
	r = p.expect(0, "run", "--dangerously-bypass-protections", "--", "sh", "-c", `cat "$1"; echo; cat "$1" >&2`, "sh", keyFile)
	if strings.Contains(r.out(), sk) || strings.Contains(r.out(), strings.TrimPrefix(sk, "zeze-sk-1:")) || !strings.Contains(r.stdout, "[REDACTED]") {
		t.Fatalf("private key not redacted in child output (bypass must not disable it):\n%s", r)
	}
}

func TestRunRedaction(t *testing.T) {
	needSh(t)
	p := newProject(t)
	p.setup(`{"split":"abcdefghijkl","short":"xyz","pin":4242,"tiny":"q","num":777,"url":"a b/c?d=e&f"}`)
	p.write("config.schema.json", `{"type":"object","properties":{"pin":{"type":"number","x-zeze-secret":true},"tiny":{"type":"string","x-zeze-secret":true}}}`)

	// A value split across two writes (and across stdout/stderr flushes) is still caught.
	r := p.expect(0, "run", "--", "sh", "-c", `printf 'pre-abcdef'; sleep 0.3; printf 'ghijkl-post\n'; printf 'abc' >&2; sleep 0.2; printf 'defghijkl\n' >&2`)
	if r.stdout != "pre-[REDACTED]-post\n" || r.stderr != "[REDACTED]\n" {
		t.Fatalf("split redaction: stdout %q stderr %q", r.stdout, r.stderr)
	}
	// Short strings are not redacted; x-zeze-secret values of any type and length are.
	r = p.expect(0, "run", "--format", "flatten", "--", "sh", "-c", `printf '%s|%s|%s|%s' "$SHORT" "$PIN" "$TINY" "$NUM"`)
	if r.stdout != "xyz|[REDACTED]|[REDACTED]|777" {
		t.Fatalf("secret redaction: %q", r.stdout)
	}
	// Encoded variants are redacted too.
	b64 := base64.StdEncoding.EncodeToString([]byte("abcdefghijkl"))
	qe := url.QueryEscape("a b/c?d=e&f")
	r = p.expect(0, "run", "--", "sh", "-c", `printf '%s\n' "$1" "$2"`, "sh", b64, qe)
	if strings.Contains(r.stdout, b64) || strings.Contains(r.stdout, qe) || strings.Count(r.stdout, "[REDACTED]") != 2 {
		t.Fatalf("encoded variants not redacted: %q", r.stdout)
	}
	// JSON-escaped forms inside ZEZE_CONFIG are redacted too.
	p.expect(0, "set", "/esc", `q"uo\\te/é`)
	r = p.expect(0, "run", "--", "sh", "-c", `printf '%s' "$ZEZE_CONFIG"`)
	if strings.Contains(r.stdout, `uo\\`) || !strings.Contains(r.stdout, `"esc":"[REDACTED]"`) {
		t.Fatalf("JSON-escaped value not redacted: %s", r.stdout)
	}
	// With the bypass values are shown.
	r = p.expect(0, "run", "--dangerously-bypass-protections", "--format", "flatten", "--", "sh", "-c", `printf '%s|%s' "$SPLIT" "$PIN"`)
	if r.stdout != "abcdefghijkl|4242" {
		t.Fatalf("bypass output: %q", r.stdout)
	}
}
