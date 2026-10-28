//go:build !js

package main

import (
	"os"
	"strings"
)

// Desktop links can be pasted in-app until OS deep-link registration is added.
// An explicit argument also permits a launcher to hand over a received link.
func startupAccountLink() string {
	for i, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "--account-link=") {
			return strings.TrimPrefix(arg, "--account-link=")
		}
		if arg == "--account-link" && i+2 < len(os.Args) {
			return os.Args[i+2]
		}
	}
	return ""
}

func watchAccountLinks(func(string)) {}
