// Command quicknavgen writes internal/appui/quicknav_settings_gen.go, the
// settings the quick navigation palette searches. Run it from anywhere in
// the repository:
//
//	go generate ./internal/appui
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/himasifunej/himasif-space/tools/quicknavgen"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	root, err := quicknavgen.Root(wd)
	if err != nil {
		log.Fatal(err)
	}
	src, err := quicknavgen.Generate(root)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "appui", "quicknav_settings_gen.go"), src, 0o644); err != nil {
		log.Fatal(err)
	}
}
