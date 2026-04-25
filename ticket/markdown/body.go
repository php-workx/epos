package markdown

import (
	"strings"
	"time"
)

// ExtractHeadingTitle scans body for the first top-level Markdown heading (a line
// beginning with exactly one '#' followed by a space) and returns its text.
// The boolean result is false when no such heading is found.
func ExtractHeadingTitle(body string) (string, bool) {
	for _, line := range strings.Split(body, "\n") {
		// Trim any trailing carriage return for Windows-style line endings.
		line = strings.TrimRight(line, "\r")

		// Must start with exactly "# " (H1 only).
		if !strings.HasPrefix(line, "# ") {
			continue
		}
		title := strings.TrimSpace(line[2:])
		if title != "" {
			return title, true
		}
	}
	return "", false
}

// AddNote appends a timestamped note to the ## Notes section of body.
// If no ## Notes section exists one is created at the end of the body.
// The timestamp is formatted in RFC 3339 UTC.
//
// The note text is formatted as a Markdown list item:
//
//   - 2006-01-02T15:04:05Z: <note>
func AddNote(body, note string) string {
	ts := time.Now().UTC().Format(time.RFC3339)
	entry := "- " + ts + ": " + note

	const heading = "## Notes"

	// Locate the ## Notes section. We track the absolute byte offset at which the
	// heading line begins, using a boolean to distinguish "found at offset 0" from
	// "not found" (which a single -1 sentinel cannot express unambiguously).
	headingStart := -1

	if idx := strings.Index(body, "\n"+heading); idx != -1 {
		// Heading is preceded by a newline.
		headingStart = idx + 1
	} else if strings.HasPrefix(strings.TrimLeft(body, "\r\n"), heading) {
		// Heading is at the very start of body (no preceding newline).
		headingStart = strings.Index(body, heading)
	}

	if headingStart == -1 {
		// No Notes section found — append one.
		if len(body) > 0 && !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		body += "\n" + heading + "\n\n" + entry + "\n"
		return body
	}

	// Find the end of the heading line.
	headingEnd := strings.Index(body[headingStart:], "\n")
	if headingEnd == -1 {
		// Heading is the last line — just append.
		if !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		return body + "\n" + entry + "\n"
	}
	sectionStart := headingStart + headingEnd + 1 // absolute position just after heading newline

	// Find the next ## heading that ends this section.
	rest := body[sectionStart:]
	nextHeadingIdx := strings.Index(rest, "\n## ")
	if nextHeadingIdx == -1 {
		// Notes is the last section — append at the end.
		if !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		return body + entry + "\n"
	}

	// Insert the entry just before the next section.
	insertAt := sectionStart + nextHeadingIdx + 1 // +1 to keep the \n before next ##
	return body[:insertAt] + entry + "\n" + body[insertAt:]
}
