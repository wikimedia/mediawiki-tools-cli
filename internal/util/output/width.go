package output

// truncateCell truncates s to maxWidth runes, appending '…' when truncated.
// maxWidth <= 0 means no truncation.
func truncateCell(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	if maxWidth == 1 {
		return "…"
	}
	return string(runes[:maxWidth-1]) + "…"
}
