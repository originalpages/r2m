// Command r2m generates Mermaid diagrams from a folder or GitHub repository.
package main

import (
	"bufio"
	"fmt"
	"os"
	"runtime"

	"github.com/originalpages/r2m/internal/cli"
)

func isTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

func main() {
	interactive := isTerminal(os.Stdout) && isTerminal(os.Stdin)
	stdin := bufio.NewReader(os.Stdin)
	code := cli.Run(os.Args[1:], cli.IO{
		Stdin:       stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
		Interactive: interactive,
	})
	// Started by double-clicking on Windows: the console would close before it can be read.
	if runtime.GOOS == "windows" && interactive && len(os.Args) == 1 {
		fmt.Print("\nPress Enter to close this window...")
		_, _ = stdin.ReadString('\n')
	}
	os.Exit(code)
}
