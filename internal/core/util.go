package core

import "strings"

// ICU root collation order for ASCII punctuation/symbols (they sort before digits and letters).
const punctOrder = " _-,;:!?.'\"()[]{}@*/\\&#%`^+<=>|~$"

func collationRank(r rune) int {
	switch {
	case r < 128 && strings.ContainsRune(punctOrder, r):
		return strings.IndexRune(punctOrder, r)
	case r >= '0' && r <= '9':
		return 100 + int(r-'0')
	case r >= 'a' && r <= 'z':
		return 200 + int(r-'a')
	case r < 128:
		return 50 // other control/ASCII
	}
	return 1000 + int(r)
}

// LocaleCompare approximates JavaScript's localeCompare (ICU root collation) for ordering:
// punctuation < digits < letters, case-insensitive first, then lowercase before uppercase.
// Returns <0, 0 or >0.
func LocaleCompare(a, b string) int {
	ra, rb := []rune(strings.ToLower(a)), []rune(strings.ToLower(b))
	for i := 0; i < len(ra) && i < len(rb); i++ {
		if ra[i] != rb[i] {
			if collationRank(ra[i]) < collationRank(rb[i]) {
				return -1
			}
			return 1
		}
	}
	if len(ra) != len(rb) {
		if len(ra) < len(rb) {
			return -1
		}
		return 1
	}
	// Same letters, different case: lowercase sorts first.
	return -strings.Compare(a, b)
}
