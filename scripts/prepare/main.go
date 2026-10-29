// Command prepare builds the frontend. GoReleaser runs it before the Windows
// binary. The APK is built by the Ubuntu release job; the Windows image sets
// ANDROID_HOME, and the Wails Android task refuses that host.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	root := rootDir()
	run(root, "wails3", "generate", "bindings", "-ts", "./...")
	run(filepath.Join(root, "frontend"), "npm", "ci")
	run(filepath.Join(root, "frontend"), "npm", "run", "build")
}

func rootDir() string {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	if err != nil {
		return "."
	}
	return string(trim(out))
}

func trim(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}

func run(dir, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s %v: %v\n", name, args, err)
		os.Exit(1)
	}
}
