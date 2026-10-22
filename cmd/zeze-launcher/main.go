// Command zeze-launcher is the stable launcher installed at
// $HOME/.guiho/bin/zeze (zeze.exe on Windows). It reads the active
// version from ~/.guiho/zeze/current.json and starts
// versions/<active>/zeze with the original arguments and standard streams,
// returning the payload's exact exit code. When the active payload is missing
// or cannot be started, it falls back to the retained previous payload. The
// launcher is release-independent: it carries no version of its own and never
// alters its arguments.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/CGuiho/zeze/internal/paths"
)

const currentFile = "current.json"

// maxPointerSize bounds current.json; the installers write one short line.
const maxPointerSize = 4096

// semverPattern matches the exact SemVer form the installers and the upgrade
// transaction write into current.json.
var semverPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)

// pointer is the strictly decoded active-version pointer: the target version
// and the retained previous version, both as relative names inside the
// versions directory.
type pointer struct{ Active, Previous string }

// starter starts payload with args and returns the payload's exit code. It
// returns an error when the payload cannot be started. On Unix the real
// delegate replaces this process (exec) and never returns on success.
type starter func(payload string, args []string) (int, error)

func main() {
	os.Exit(run(os.Args[1:], delegate, os.Stderr))
}

// payloadName is the platform payload file name inside a version directory.
func payloadName() string {
	if runtime.GOOS == "windows" {
		return "zeze.exe"
	}
	return "zeze"
}

// reinstallHint is printed with every launcher failure so the reader can
// repair the installation.
func reinstallHint() string {
	if runtime.GOOS == "windows" {
		return "reinstall: irm https://raw.githubusercontent.com/CGuiho/zeze/main/devops/install.ps1 | iex"
	}
	return "reinstall: curl -fsSL https://raw.githubusercontent.com/CGuiho/zeze/main/devops/install.sh | sh"
}

func fail(errOut io.Writer, format string, args ...any) int {
	fmt.Fprintf(errOut, "zeze launcher: "+format+"\n", args...)
	return 1
}

// run resolves the CLI home, reads current.json and starts the active payload,
// falling back to the previous payload when the active one is missing or
// cannot be started. It returns the payload's exit code, or 1 when no payload
// could be started.
func run(args []string, start starter, errOut io.Writer) int {
	home, err := paths.Home()
	if err != nil {
		return fail(errOut, "cannot resolve the zeze home: %v; %s", err, reinstallHint())
	}
	p, err := readPointer(filepath.Join(home, currentFile))
	if err != nil {
		return fail(errOut, "%v; %s", err, reinstallHint())
	}
	versions := filepath.Join(home, "versions")

	payload, activeErr := resolvePayload(versions, p.Active)
	if activeErr == nil {
		code, err := start(payload, args)
		if err == nil {
			return code
		}
		activeErr = fmt.Errorf("cannot start the payload: %w", err)
	}
	runErr := fmt.Errorf("the active version %s is unavailable (%v)", p.Active, activeErr)

	if p.Previous == "" {
		return fail(errOut, "%v; %s", runErr, reinstallHint())
	}
	fallback, fallbackErr := resolvePayload(versions, p.Previous)
	if fallbackErr != nil {
		return fail(errOut, "%v; the fallback version %s is unavailable (%v); %s", runErr, p.Previous, fallbackErr, reinstallHint())
	}
	fmt.Fprintf(errOut, "zeze launcher: warning: %v; running the fallback version %s\n", runErr, p.Previous)
	if code, err := start(fallback, args); err == nil {
		return code
	}
	return fail(errOut, "%v; the fallback version %s cannot start (%v); %s", runErr, p.Previous, err, reinstallHint())
}

// readPointer strictly decodes current.json: exactly the three keys
// protocol, active and previous, exactly one JSON document, protocol 1, and
// version names that are valid SemVer and safe relative names inside the
// versions directory. previous may be null.
func readPointer(path string) (pointer, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return pointer{}, errors.New("current.json is missing or unreadable (no active installation)")
	}
	if len(data) == 0 || len(data) > maxPointerSize {
		return pointer{}, errors.New("current.json is malformed")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil {
		return pointer{}, errors.New("current.json is malformed")
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return pointer{}, errors.New("current.json must contain exactly one JSON document")
	}
	for key := range fields {
		switch key {
		case "protocol", "active", "previous":
		default:
			return pointer{}, errors.New("current.json has unexpected fields")
		}
	}
	rawProtocol, ok := fields["protocol"]
	if !ok {
		return pointer{}, errors.New("current.json has no protocol")
	}
	var protocol int
	if err := json.Unmarshal(rawProtocol, &protocol); err != nil {
		return pointer{}, errors.New("current.json is malformed")
	}
	if protocol != 1 {
		return pointer{}, errors.New("current.json protocol must be 1")
	}
	rawActive, ok := fields["active"]
	if !ok {
		return pointer{}, errors.New("current.json has no active version")
	}
	active, err := decodeVersion(rawActive, "active", false)
	if err != nil {
		return pointer{}, err
	}
	rawPrevious, ok := fields["previous"]
	if !ok {
		return pointer{}, errors.New("current.json has no previous field (use null)")
	}
	previous, err := decodeVersion(rawPrevious, "previous", true)
	if err != nil {
		return pointer{}, err
	}
	return pointer{Active: active, Previous: previous}, nil
}

// decodeVersion decodes one version string and validates it as SemVer and as a
// safe relative name inside versions/. A null value is accepted only when
// nullable (the previous version may be null).
func decodeVersion(raw json.RawMessage, field string, nullable bool) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("current.json is malformed (%s)", field)
	}
	if string(raw) == "null" {
		if nullable {
			return "", nil
		}
		return "", fmt.Errorf("current.json %s version must be a string", field)
	}
	var value string
	if raw[0] != '"' || json.Unmarshal(raw, &value) != nil {
		return "", fmt.Errorf("current.json %s version must be a string", field)
	}
	if value == "" {
		return "", fmt.Errorf("current.json %s version is empty", field)
	}
	if !semverPattern.MatchString(value) {
		return "", fmt.Errorf("current.json %s version is not a valid SemVer", field)
	}
	if err := validateRelative(value); err != nil {
		return "", fmt.Errorf("current.json %s version: %w", field, err)
	}
	return value, nil
}

// validateRelative rejects anything that could escape the versions directory:
// absolute paths, volume names, drive letters, NUL bytes and ".." components.
func validateRelative(value string) error {
	normalized := strings.ReplaceAll(value, `\`, "/")
	if strings.TrimSpace(value) == "" || filepath.IsAbs(value) || filepath.VolumeName(value) != "" ||
		strings.ContainsRune(value, ':') || strings.ContainsRune(value, 0) || strings.HasPrefix(normalized, "/") {
		return errors.New("the version name must be a relative path")
	}
	for _, component := range strings.Split(normalized, "/") {
		if component == ".." {
			return errors.New("the version name traverses the versions directory")
		}
	}
	clean := filepath.Clean(filepath.FromSlash(normalized))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return errors.New("the version name traverses the versions directory")
	}
	return nil
}

// resolvePayload maps a version name to versions/<name>/zeze(.exe) and
// refuses any target that is not a regular file strictly inside the versions
// directory.
func resolvePayload(versionsRoot, version string) (string, error) {
	if err := validateRelative(version); err != nil {
		return "", err
	}
	root, err := filepath.Abs(versionsRoot)
	if err != nil {
		return "", err
	}
	if resolvedRoot, err := filepath.EvalSymlinks(root); err == nil {
		root = filepath.Clean(resolvedRoot)
	}
	target := filepath.Join(root, filepath.FromSlash(strings.ReplaceAll(version, `\`, "/")), payloadName())
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", fmt.Errorf("the payload %s is missing", version)
	}
	rel, err := filepath.Rel(root, filepath.Clean(resolved))
	if err != nil || rel == "." || rel == ".." || filepath.IsAbs(rel) || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("the payload resolves outside the versions directory")
	}
	info, err := os.Lstat(target)
	if err != nil {
		return "", fmt.Errorf("the payload %s is missing", version)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("the payload is a symlink")
	}
	if !info.Mode().IsRegular() {
		return "", errors.New("the payload is not a regular file")
	}
	return target, nil
}
