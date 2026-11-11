//go:build spacepprof && !js

// Built with -tags spacepprof, the desktop app serves net/http/pprof on
// SPACE_PPROF (default 127.0.0.1:6061) to measure its memory:
//
//	go build -tags spacepprof -o build/space-pprof.exe ./app
//	go tool pprof -top http://127.0.0.1:6061/debug/pprof/heap
//
// Normal builds leave this file out.
package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
)

func init() {
	addr := os.Getenv("SPACE_PPROF")
	if addr == "" {
		addr = "127.0.0.1:6061"
	}
	go func() { log.Println("pprof:", http.ListenAndServe(addr, nil)) }()
}
