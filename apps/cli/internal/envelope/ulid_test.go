package envelope

import (
	"strings"
	"testing"
	"time"
)

// TestULIDLengthAndAlphabet pins the wire format: 26 chars, every
// character drawn from Crockford base32. Anything else means the
// encoder drifted from the spec and consumers expecting a
// fixed-width id will choke.
func TestULIDLengthAndAlphabet(t *testing.T) {
	for i := 0; i < 100; i++ {
		id := NewULID()
		if len(id) != 26 {
			t.Fatalf("ULID length = %d, want 26: %q", len(id), id)
		}
		for _, r := range id {
			if !strings.ContainsRune(crockfordAlphabet, r) {
				t.Fatalf("ULID contains non-Crockford rune %q in %q", r, id)
			}
		}
	}
}

// TestULIDIsTimeSortable pins the load-bearing property that makes
// ULIDs valuable as run identifiers and log directory names: two
// ULIDs generated >1 ms apart are lexicographically ordered. If this
// breaks, log directories no longer chronologically sort.
func TestULIDIsTimeSortable(t *testing.T) {
	first := NewULID()
	time.Sleep(2 * time.Millisecond)
	second := NewULID()
	if first >= second {
		t.Fatalf("expected first < second lexicographically; got %q >= %q", first, second)
	}
}

// TestULIDIsUnique pins the entropy half: 100 ULIDs generated in a
// tight loop must be distinct. Same-millisecond collisions are
// allowed by the spec (timestamp prefix collides) but the random
// suffix must keep the full string unique.
func TestULIDIsUnique(t *testing.T) {
	seen := make(map[string]struct{}, 100)
	for i := 0; i < 100; i++ {
		id := NewULID()
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate ULID after %d iterations: %q", i, id)
		}
		seen[id] = struct{}{}
	}
}

// TestCrockfordEncodeDeterministic pins encoder purity: same input,
// same output, no hidden state. Necessary for the streaming golden
// transcripts where we feed a known byte pattern to validate the
// encoded form character-by-character.
func TestCrockfordEncodeDeterministic(t *testing.T) {
	in := [16]byte{
		0x01, 0x86, 0xa3, 0x69, 0x70, 0x77,
		0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0x11, 0x22,
	}
	a := crockfordEncode(in)
	b := crockfordEncode(in)
	if a != b {
		t.Fatalf("crockfordEncode is not pure: %q vs %q", a, b)
	}
	if len(a) != 26 {
		t.Fatalf("encoded length = %d, want 26: %q", len(a), a)
	}
}
