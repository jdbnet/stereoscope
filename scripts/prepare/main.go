// Command prepare builds the frontend and, when the Android SDK is present,
// the release APK. GoReleaser runs it before compiling the Windows binary.
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
	if os.Getenv("ANDROID_HOME") == "" && os.Getenv("ANDROID_SDK_ROOT") == "" {
		fmt.Println("prepare: Android SDK not configured, skipping the APK")
		return
	}
	run(root, "wails3", "task", "android:package")
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
