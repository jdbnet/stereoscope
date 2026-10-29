// Command package copies an amd64 Windows binary into place and builds the NSIS installer.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: package <binary>")
		os.Exit(1)
	}
	binary := os.Args[1]
	if !strings.Contains(strings.ToLower(binary), "amd64") {
		fmt.Println("package: skipping NSIS for", binary)
		return
	}
	if _, err := exec.LookPath("makensis"); err != nil {
		fmt.Println("package: makensis not installed, skipping the installer")
		return
	}
	root := moduleRoot()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o755); err != nil {
		exit(err)
	}
	dest := filepath.Join(root, "bin", "stereoscope.exe")
	if err := copyFile(binary, dest); err != nil {
		exit(err)
	}
	run(root, "wails3", "generate", "webview2bootstrapper", "-dir", filepath.Join(root, "build", "windows", "nsis"))
	run(filepath.Join(root, "build", "windows", "nsis"), "makensis", "-DARG_WAILS_AMD64_BINARY="+dest, "project.nsi")
}

func moduleRoot() string {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	if err != nil {
		return "."
	}
	return strings.TrimSpace(string(out))
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func run(dir, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		exit(err)
	}
}

func exit(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
