package domain

import (
	"regexp"
	"strconv"
	"strings"
)

var semverRegex = regexp.MustCompile(`^v?(\d+)(?:\.(\d+))?(?:\.(\d+))?`)

// ParseVersion extracts major, minor, patch ints from a version string.
func ParseVersion(v string) (major, minor, patch int, ok bool) {
	v = strings.TrimSpace(v)
	matches := semverRegex.FindStringSubmatch(v)
	if len(matches) < 2 || matches[1] == "" {
		return 0, 0, 0, false
	}

	major, _ = strconv.Atoi(matches[1])

	if len(matches) > 2 && matches[2] != "" {
		minor, _ = strconv.Atoi(matches[2])
	}

	if len(matches) > 3 && matches[3] != "" {
		patch, _ = strconv.Atoi(matches[3])
	}

	return major, minor, patch, true
}

// ClassifyChange determines whether a version transition is patch, minor, or major.
func ClassifyChange(current, latest string) (ChangeType, bool) {
	cMaj, cMin, cPatch, cOk := ParseVersion(current)
	lMaj, lMin, lPatch, lOk := ParseVersion(latest)

	if !cOk || !lOk {
		return ChangeUnknown, false
	}

	if lMaj > cMaj {
		return ChangeMajor, false // Major is breaking -> safe = false
	}

	if lMaj == cMaj && lMin > cMin {
		return ChangeMinor, true // Minor is safe
	}

	if lMaj == cMaj && lMin == cMin && lPatch > cPatch {
		return ChangePatch, true // Patch is safe
	}

	if lMaj == cMaj && lMin == cMin && lPatch == cPatch {
		return ChangePatch, true
	}

	return ChangeUnknown, false
}
