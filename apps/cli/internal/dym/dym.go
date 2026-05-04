// Package dym ("did you mean") finds the closest match for an unknown
// CLI token against a small candidate set, using Levenshtein distance.
// Used by the main.go dispatcher when the user mistypes a subcommand
// or a `dots explain` topic.
//
// The package is small on purpose: the dispatcher already knows the
// candidate slice, so dym only owns the distance metric and the
// best-match selection.
package dym

// Suggest returns the candidate closest to input by Levenshtein
// distance, plus a found bit. found is false when the closest match
// is farther than maxDistance — there is no useful suggestion to
// make. maxDistance of 3 is the conventional ceiling for CLI typo
// correction; "isntall" → "install" is a 2; anything farther tends
// to be a different word.
func Suggest(input string, candidates []string, maxDistance int) (string, bool) {
	best := ""
	bestDist := maxDistance + 1
	for _, c := range candidates {
		d := distance(input, c)
		if d < bestDist {
			best = c
			bestDist = d
		}
	}
	if bestDist > maxDistance {
		return "", false
	}
	return best, true
}

// distance returns the Levenshtein edit distance between a and b.
// Implementation is the standard two-row DP — O(len(a)*len(b)) time
// with O(min(a,b)) space. Sufficient for short inputs (CLI tokens
// rarely exceed 16 chars).
func distance(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
