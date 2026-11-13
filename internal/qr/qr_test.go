package qr

import (
	"bufio"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

// testdata/golden.txt holds symbols made by python-qrcode 8 (level M,
// byte mode, a fixed mask): text, mask, version and the modules row by row.
func TestMatchesReferenceEncoder(t *testing.T) {
	f, err := os.Open("testdata/golden.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(nil, 1<<20)
	n := 0
	for s.Scan() {
		parts := strings.Split(s.Text(), "\t")
		var text string
		if err := json.Unmarshal([]byte(parts[0]), &text); err != nil {
			t.Fatal(err)
		}
		mask, _ := strconv.Atoi(parts[1])
		version, _ := strconv.Atoi(parts[2])
		q, err := EncodeMask([]byte(text), mask)
		if err != nil {
			t.Fatalf("%q: %v", text, err)
		}
		if q.Version != version {
			t.Fatalf("%q: version %d, want %d", text, q.Version, version)
		}
		var b strings.Builder
		for y := 0; y < q.Size; y++ {
			for x := 0; x < q.Size; x++ {
				if q.Dark(x, y) {
					b.WriteByte('1')
				} else {
					b.WriteByte('0')
				}
			}
		}
		if got := b.String(); got != parts[3] {
			for i := range got {
				if got[i] != parts[3][i] {
					t.Fatalf("%q: module %d,%d differs", text, i%q.Size, i/q.Size)
				}
			}
		}
		n++
	}
	if n < 4 {
		t.Fatalf("only %d golden symbols", n)
	}
}

func TestChoosesAMaskAndRejectsTooLong(t *testing.T) {
	q, err := Encode([]byte("https://docs.google.com/spreadsheets/d/abc/edit"))
	if err != nil || q.Mask < 0 || q.Mask > 7 {
		t.Fatalf("%v %+v", err, q)
	}
	img := q.Image(4, 4)
	if img.Bounds().Dx() != (q.Size+8)*4 {
		t.Fatalf("image size %v", img.Bounds())
	}
	if _, err := Encode(make([]byte, 700)); err != ErrTooLong {
		t.Fatalf("700 bytes: %v", err)
	}
}
