package helper

// TruncateRunes bounds a string to max runes, used for chat descriptions.
func TruncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
