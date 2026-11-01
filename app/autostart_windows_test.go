//go:build windows

package main

import (
	"testing"

	"github.com/himasifunej/himasif-space/internal/id"
	"golang.org/x/sys/windows/registry"
)

func TestWindowsAutostartQuotesPathRebindsAndRemovesOnlySpace(t *testing.T) {
	// Exercise real registry calls under a private test key, never the user's Run key.
	path := `Software\HIMASIFSpaceTests\` + id.New().String()
	k, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.ALL_ACCESS)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { k.Close(); registry.DeleteKey(registry.CURRENT_USER, path) })
	if err := k.SetStringValue("Other app", "keep"); err != nil {
		t.Fatal(err)
	}
	for _, exe := range []string{`C:\Users\Pengurus\Old folder\Space.exe`, `C:\Users\Pengurus\New folder\Space.exe`} {
		if err := writeDeviceAutostart(k, exe, true); err != nil {
			t.Fatal(err)
		}
		got, _, err := k.GetStringValue("HIMASIF Space")
		if err != nil || got != `"`+exe+`" --background` {
			t.Fatalf("incorrect startup command: %q, %v", got, err)
		}
	}
	for range 2 {
		if err := writeDeviceAutostart(k, "", false); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := k.GetStringValue("HIMASIF Space"); err != registry.ErrNotExist {
		t.Fatalf("startup entry remains: %v", err)
	}
	if got, _, err := k.GetStringValue("Other app"); err != nil || got != "keep" {
		t.Fatal("disabling Space changed another app")
	}
}
