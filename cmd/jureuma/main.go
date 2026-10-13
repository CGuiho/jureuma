// Command jureuma is the agent-safe configuration CLI.
package main

import (
	"os"

	"github.com/CGuiho/jureuma/internal/cli"
)

// version is set at build time with -ldflags "-X main.version=<semver>".
var version = "0.0.0-dev"

func main() {
	os.Exit(cli.Execute(version))
}
