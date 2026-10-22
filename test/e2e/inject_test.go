package e2e

import (
	"strings"
	"testing"
)

// `inject` is a command alias of `run` (issue #9): the same command, flags,
// injection, redaction and exit codes, only a different way to spell it.

func TestInjectAliasBehavesLikeRun(t *testing.T) {
	needSh(t)
	p := newProject(t)
	p.setup(`{"database":{"url":"postgres://u:pw@h/db","pool":20},"apiKey":"key-inject-alias"}`)

	// The child receives the injected document (checked inside the child).
	p.expect(0, "inject", "--", "sh", "-c", `case "$ZEZE_CONFIG" in *'"apiKey":"key-inject-alias"'*) exit 0;; *) exit 1;; esac`)
	// The exact document reaches the child.
	r := p.expect(0, "inject", "--dangerously-bypass-protections", "--", "sh", "-c", `printf '%s' "$ZEZE_CONFIG"`)
	if r.stdout != `{"database":{"url":"postgres://u:pw@h/db","pool":20},"apiKey":"key-inject-alias"}` {
		t.Fatalf("ZEZE_CONFIG = %s", r.stdout)
	}
	// Child output is redacted exactly like run.
	r = p.expect(0, "inject", "--", "sh", "-c", `printf '%s' "$ZEZE_CONFIG"`)
	if r.stdout != `{"database":{"url":"[REDACTED]","pool":20},"apiKey":"[REDACTED]"}` {
		t.Fatalf("inject redaction: %s", r.stdout)
	}
	assertNo(t, "inject output", r.out(), "postgres://u:pw@h/db", "key-inject-alias")
	// Every run flag is accepted: flatten, prefix, env, env-name.
	r = p.expect(0, "inject", "--dangerously-bypass-protections", "--format", "flatten", "--prefix", "APP_",
		"--env", "DB=/database/url", "--", "sh", "-c", `printf '%s|%s|%s' "$APP_DATABASE_POOL" "$DB" "${ZEZE_CONFIG-unset}"`)
	if r.stdout != "20|postgres://u:pw@h/db|unset" {
		t.Fatalf("inject flags: %q", r.stdout)
	}
	r = p.expect(0, "inject", "--env-name", "APP_CFG", "--", "sh", "-c", `printf '%s|%s' "${APP_CFG:+set}" "${ZEZE_CONFIG:-unset}"`)
	if r.stdout != "set|unset" {
		t.Fatalf("inject --env-name: %q", r.stdout)
	}
}

func TestInjectAliasExitCodes(t *testing.T) {
	needSh(t)
	p := newProject(t)
	p.setup(`{"a":"inject-exit-value"}`)
	// The child's exit code passes through, like run.
	for _, c := range []string{"0", "1", "7", "42", "124"} {
		r := p.run("inject", "--", "sh", "-c", "exit "+c)
		if r.stderr != "" || itoa(r.code) != c {
			t.Fatalf("inject child exit %s passed through as %d\n%s", c, r.code, r)
		}
	}
	// zeze-side failures keep the run contract: 125 with a [ZEZE:<KIND>]
	// prefix (cmd.Name() stays "run" under the alias).
	r := p.run("inject", "--format", "yaml", "--", "true")
	if r.code != 125 || !strings.HasPrefix(r.stderr, "[ZEZE:USAGE]") {
		t.Fatalf("inject bad --format: want 125 [ZEZE:USAGE]\n%s", r)
	}
	r = p.run("inject", "--", "definitely-not-a-command-e2e")
	if r.code != 125 || !strings.HasPrefix(r.stderr, "[ZEZE:") {
		t.Fatalf("inject missing command: want 125\n%s", r)
	}
	assertNo(t, "inject failure output", r.out(), "inject-exit-value")
}

func TestInjectAliasIsDiscoverable(t *testing.T) {
	p := newProject(t)
	// run --help advertises the alias (Cobra's Aliases line).
	r := p.expect(0, "run", "--help")
	if !strings.Contains(r.stdout, "inject") {
		t.Fatalf("run --help does not mention the inject alias:\n%s", r.stdout)
	}
	// inject --help is the run help.
	a := p.expect(0, "inject", "--help")
	if !strings.Contains(a.stdout, "zeze run") || !strings.Contains(a.stdout, "--format") {
		t.Fatalf("inject --help is not the run help:\n%s", a.stdout)
	}
	if r := p.expect(0, "help", "inject"); !strings.Contains(r.stdout, "zeze run") {
		t.Fatalf("help inject is not the run help:\n%s", r.stdout)
	}
	// --help-docs mentions the alias and is identical for both spellings.
	d := p.expect(0, "run", "--help-docs")
	if !strings.Contains(d.stdout, "inject") {
		t.Fatalf("run --help-docs does not mention the inject alias:\n%s", d.stdout)
	}
	if ad := p.expect(0, "inject", "--help-docs"); ad.stdout != d.stdout {
		t.Fatalf("inject --help-docs differs from run --help-docs:\n%s\n%s", d.stdout, ad.stdout)
	}
	// The tree lists command names only: the alias is not a separate node.
	rt := p.expect(0, "run", "--help-tree")
	if it := p.expect(0, "inject", "--help-tree"); it.stdout != rt.stdout {
		t.Fatalf("inject --help-tree differs from run --help-tree:\n%s\n%s", rt.stdout, it.stdout)
	}
	root := p.expect(0, "--help-tree")
	if strings.Contains(root.stdout, "── inject") {
		t.Fatalf("help tree must not add an inject node:\n%s", root.stdout)
	}
	if !strings.Contains(root.stdout, "── run") {
		t.Fatalf("help tree lost the run node:\n%s", root.stdout)
	}
}
