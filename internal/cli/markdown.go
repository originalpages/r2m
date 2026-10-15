package cli

import "strings"

const (
	startMarker = "<!-- r2m:start -->"
	endMarker   = "<!-- r2m:end -->"
)

// UpdateMarkerBlock replaces the marker block content with a mermaid fence, or appends the
// block when the markers are absent.
func UpdateMarkerBlock(md, mermaid string) string {
	block := startMarker + "\n```mermaid\n" + strings.TrimRight(mermaid, "\n") + "\n```\n" + endMarker
	if s := strings.Index(md, startMarker); s >= 0 {
		if e := strings.Index(md[s+len(startMarker):], endMarker); e >= 0 {
			end := s + len(startMarker) + e + len(endMarker)
			return md[:s] + block + md[end:]
		}
	}
	base := strings.TrimRight(md, " \t\r\n")
	if base != "" {
		base += "\n\n"
	}
	return base + block + "\n"
}
