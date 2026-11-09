package broadcast

import "testing"

// Synthetic fixtures test the declared grammar, not acceptance of an unseen live template.
func TestExplicitListPreservesMissingYearAndAmbiguity(t *testing.T) {
	items := Parse("1. Poster A\nPJ Humas: Ana\nTanggal posting: 8 November\n2. Poster B\nPJ Humas: Budi\nPJ Humas: Citra\nTanggal: 31/02/2026")
	if len(items) != 2 {
		t.Fatalf("items %+v", items)
	}
	if items[0].PostingDate != nil || items[0].Day != 8 || items[0].Month != 11 {
		t.Fatal("missing year guessed")
	}
	if items[1].Issue == "" || items[1].PostingDate != nil {
		t.Fatal("ambiguous/invalid item treated as safe")
	}
	if got := Parse("1. Hai teman\n2. Sudah selesai"); len(got) != 0 {
		t.Fatal("conversation parsed as broadcast")
	}
}
