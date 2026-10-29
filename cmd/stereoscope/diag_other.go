//go:build !windows

package main

import "net/http"

func openDiag() func() { return func() {} }

func loggedAssets(next http.Handler) http.Handler { return next }
