// Command keenetic-xray is the single entry point for the installer, daemon,
// CLI, setup wizard, and control-server agent — dispatched by subcommand.
package main

import (
	"fmt"
	"os"

	"github.com/Kuzz007/keenetic-xray-go/internal/version"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "keenetic-xray:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}

	switch args[0] {
	case "version":
		fmt.Println(version.String())
		return nil
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return usageError()
	}
}

func usageError() error {
	printUsage()
	return fmt.Errorf("unknown or unimplemented command %q", os.Args[len(os.Args)-1])
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `usage: keenetic-xray <command> [args]

commands:
  version      print version and exit
  setup        interactive first-run configuration menu (not yet implemented)
  daemon       run the failover daemon in the foreground (not yet implemented)
  profile      manage VLESS profiles (not yet implemented)
  subscription manage the subscription URL and its profiles (not yet implemented)
  status       show current failover status (not yet implemented)
  doctor       print diagnostics (not yet implemented)
  variant      show or set the mini/full variant (not yet implemented)
  agent        run the control-server polling agent (not yet implemented)`)
}
