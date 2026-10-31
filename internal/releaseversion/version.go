// Package releaseversion compares historical semantic versions and dated releases.
package releaseversion

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/blang/semver"
)

// Version is either a semantic version or YYYY.MM.DD.REVISION. Dated releases
// follow all historical semantic releases, regardless of their major version.
type Version struct {
	text     string
	semantic semver.Version
	date     [4]uint64
	dated    bool
}

// Parse accepts an optional v prefix. Dated prereleases are not supported.
func Parse(input string) (Version, error) {
	text := strings.TrimPrefix(input, "v")
	parts := strings.Split(text, ".")
	core := strings.SplitN(strings.SplitN(text, "-", 2)[0], "+", 2)[0]
	if len(strings.Split(core, ".")) != 4 {
		v, err := semver.Parse(text)
		if err != nil {
			return Version{}, fmt.Errorf("invalid release version %q: %w", input, err)
		}
		return Version{text: text, semantic: v}, nil
	}
	if len(parts) != 4 || strings.ContainsAny(text, "-+") {
		return Version{}, fmt.Errorf("invalid dated release version %q", input)
	}
	v := Version{text: text, dated: true}
	for i, part := range parts {
		if part == "" || strings.IndexFunc(part, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return Version{}, fmt.Errorf("invalid dated release version %q", input)
		}
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return Version{}, fmt.Errorf("invalid dated release version %q: %w", input, err)
		}
		v.date[i] = n
	}
	if len(parts[0]) != 4 || v.date[0] == 0 || len(parts[1]) > 2 || len(parts[2]) > 2 || (len(parts[3]) > 1 && parts[3][0] == '0') {
		return Version{}, fmt.Errorf("invalid dated release version %q", input)
	}
	date := fmt.Sprintf("%04d-%02d-%02d", v.date[0], v.date[1], v.date[2])
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return Version{}, fmt.Errorf("invalid release date in %q: %w", input, err)
	}
	return v, nil
}

func (v Version) String() string { return v.text }

// Stable reports whether a parsed version is eligible for automatic updates.
func (v Version) Stable() bool { return v.dated || len(v.semantic.Pre) == 0 }

// Compare returns -1, 0 or 1, ignoring semantic build metadata and date padding.
func (v Version) Compare(other Version) int {
	if v.dated != other.dated {
		if v.dated {
			return 1
		}
		return -1
	}
	if !v.dated {
		return v.semantic.Compare(other.semantic)
	}
	for i, n := range v.date {
		if n < other.date[i] {
			return -1
		}
		if n > other.date[i] {
			return 1
		}
	}
	return 0
}
