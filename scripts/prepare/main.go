// Command prepare builds the frontend. GoReleaser runs it before the Windows
// binary. The APK is built by the Ubuntu release job; the Windows image sets
// ANDROID_HOME, and the Wails Android task refuses that host.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	root := rootDir()
	run(root, "wails3", "generate", "bindings", "-ts", "./...")
	run(filepath.Join(root, "frontend"), "npm", "ci")
	run(filepath.Join(root, "frontend"), "npm", "run", "build")
	if version := releaseVersion(); version != "" {
		stampWindowsVersion(root, version)
	}
}

func releaseVersion() string {
	for i, arg := range os.Args[1:] {
		if arg == "--version" && i+1 < len(os.Args[1:]) {
			return strings.TrimPrefix(os.Args[i+2], "v")
		}
	}
	return strings.TrimPrefix(os.Getenv("STEREO_VERSION"), "v")
}

func stampWindowsVersion(root, version string) {
	path := filepath.Join(root, "build", "windows", "info.json")
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read windows version info: %v\n", err)
		os.Exit(1)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		fmt.Fprintf(os.Stderr, "parse windows version info: %v\n", err)
		os.Exit(1)
	}
	if fixed, ok := doc["fixed"].(map[string]any); ok {
		fixed["file_version"] = version
	}
	if info, ok := doc["info"].(map[string]any); ok {
		if lang, ok := info["0409"].(map[string]any); ok {
			lang["ProductVersion"] = version
		}
	}
	out, err := json.MarshalIndent(doc, "", "\t")
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode windows version info: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write windows version info: %v\n", err)
		os.Exit(1)
	}
	build := filepath.Join(root, "build")
	for _, arch := range []string{"amd64", "arm64"} {
		run(build, "wails3", "generate", "syso",
			"-arch", arch,
			"-icon", "windows/icon.ico",
			"-manifest", "windows/wails.exe.manifest",
			"-info", "windows/info.json",
			"-out", filepath.Join("..", "cmd", "stereoscope", "wails_windows_"+arch+".syso"),
		)
	}
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
