package config

import "strings"

// Environment variable names jureuma never produces from configuration data
// (flatten mode, run.envName, --env-name): names that make a program load or
// run code, change how a shell or tool executes commands, or belong to
// jureuma itself. A key added by "set" (anyone holding the committed public
// key can add one) must not become code execution inside a process that
// holds every secret. "run --env NAME=/pointer" may still map them
// explicitly, because that mapping is typed on the command line.
var (
	deniedEnvExact = []string{
		// Shells.
		"BASH_ENV", "ENV", "PROMPT_COMMAND", "PS1", "PS2", "PS4", "SHELLOPTS", "BASHOPTS",
		"IFS", "CDPATH", "PATH", "SHELL", "ZDOTDIR",
		// Commands that tools run through a shell.
		"PAGER", "MANPAGER", "EDITOR", "VISUAL", "BROWSER", "LESSOPEN", "LESSCLOSE",
		"SSH_ASKPASS", "SUDO_ASKPASS",
		// Language runtimes.
		"NODE_OPTIONS", "NODE_PATH",
		"PYTHONSTARTUP", "PYTHONPATH", "PYTHONHOME",
		"PERL5OPT", "PERL5LIB", "PERLLIB",
		"RUBYOPT", "RUBYLIB",
		"_JAVA_OPTIONS", "JDK_JAVA_OPTIONS",
		"RUSTC_WRAPPER",
	}
	deniedEnvPrefixes = []string{
		"LD_", "DYLD_", // dynamic loader
		"GIT_",        // GIT_CONFIG_*, GIT_SSH_COMMAND, GIT_EXTERNAL_DIFF, GIT_PAGER, ...
		"JAVA_",       // JAVA_TOOL_OPTIONS, ...
		"NPM_CONFIG_", // npm_config_script_shell, ...
		"BASH_FUNC_",  // exported shell functions
		"JUREUMA_",    // jureuma's own variables
	}
)

// DeniedEnvName reports whether name is on the denylist (compared in upper
// case, because Windows environment names are case-insensitive).
func DeniedEnvName(name string) bool {
	up := strings.ToUpper(name)
	for _, d := range deniedEnvExact {
		if up == d {
			return true
		}
	}
	for _, p := range deniedEnvPrefixes {
		if strings.HasPrefix(up, p) {
			return true
		}
	}
	return false
}

// DeniedRunEnvName reports whether name may not carry the document in
// run's json and file formats: every denylisted name except DefaultEnvName.
func DeniedRunEnvName(name string) bool {
	return !strings.EqualFold(name, DefaultEnvName) && DeniedEnvName(name)
}
