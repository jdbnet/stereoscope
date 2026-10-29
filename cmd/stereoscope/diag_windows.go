//go:build windows

package main

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime/debug"

	"stereoscope/frontend"
)

func openDiag() func() {
	dir, err := os.Getwd()
	if err != nil || dir == "" {
		if exe, exeErr := os.Executable(); exeErr == nil {
			dir = filepath.Dir(exe)
		} else {
			dir = "."
		}
	}
	path := filepath.Join(dir, "stereoscope.log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return func() {}
	}
	log.SetOutput(file)
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.Printf("log file: %s", path)
	if wd, wdErr := os.Getwd(); wdErr == nil {
		log.Printf("working directory: %s", wd)
	}
	if exe, exeErr := os.Executable(); exeErr == nil {
		log.Printf("executable: %s", exe)
	}
	log.Printf("dev server: %q", os.Getenv("FRONTEND_DEVSERVER_URL"))
	logEmbedded()
	return func() { _ = file.Close() }
}

func logEmbedded() {
	err := fs.WalkDir(frontend.Dist(), ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			log.Printf("embed %s: %v", path, err)
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			log.Printf("embed %s", path)
			return nil
		}
		log.Printf("embed %s (%d bytes)", path, info.Size())
		return nil
	})
	if err != nil {
		log.Printf("embed walk: %v", err)
	}
}

func loggedAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := httptest.NewRecorder()
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("asset panic %s %s: %v\n%s", r.Method, r.URL.RequestURI(), recovered, debug.Stack())
				http.Error(w, fmt.Sprint(recovered), http.StatusInternalServerError)
			}
		}()
		if r.URL.Path == "/__diag" {
			body, _ := io.ReadAll(io.LimitReader(r.Body, 8192))
			log.Printf("script: %s", bytes.TrimSpace(body))
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(rec, r)
		log.Printf("asset %s %s -> %d %d bytes %s", r.Method, r.URL.RequestURI(), rec.Code, rec.Body.Len(), rec.Header().Get("Content-Type"))
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			body := rec.Body.Bytes()
			log.Printf("index.html has script tag: %t, %d bytes", bytes.Contains(body, []byte("<script")), len(body))
		}
		for key, values := range rec.Header() {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(rec.Code)
		_, _ = io.Copy(w, rec.Body)
	})
}

