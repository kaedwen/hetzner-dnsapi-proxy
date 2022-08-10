package update

import (
	"strings"
)

func isSubDomain(sub, parent string) bool {
	// Parent domain must be a wildcard domain
	if parent[0] != '*' {
		return false
	}

	parentParts := strings.Split(parent, ".")
	subParts := strings.Split(sub, ".")

	// The subdomain must have at least the same amount of parts as the parent domain
	if len(subParts) < len(parentParts) {
		return false
	}

	// All domain parts up to the asterisk must match
	subPartsOffset := len(subParts) - len(parentParts)
	for i := len(parentParts) - 1; i > 0; i-- {
		if parentParts[i] != subParts[i+subPartsOffset] {
			return false
		}
	}

	return true
}
