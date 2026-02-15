package internal

import (
	"strconv"
)

// MaxSizeSubjectsListForLog returns the keys in the map as a slice of maximum length
// maxToDisplay. It is useful for logging domains being managed, for example, since a
// map is typically needed for quick lookup, but a slice is needed for logging, and this
// can be quite a doozy since there may be a huge amount (hundreds of thousands).
// MaxSizeSubjectsListForLog extracts up to maxToDisplay keys from the map for logging.
// It pre-allocates memory to avoid extra slice growth and avoids fmt reflection
// for optimal performance in high-throughput environments.
func MaxSizeSubjectsListForLog(subjects map[string]struct{}, maxToDisplay int) []string {
	total := len(subjects)
	if total == 0 {
		return nil
	}

	limit := min(total, maxToDisplay)

	capacity := limit
	if total > maxToDisplay {
		capacity++
	}

	domainsToDisplay := make([]string, 0, capacity)

	for domain := range subjects {
		domainsToDisplay = append(domainsToDisplay, domain)
		if len(domainsToDisplay) == limit {
			break
		}
	}

	if total > maxToDisplay {
		msg := "(and " + strconv.Itoa(total-maxToDisplay) + " more...)"
		domainsToDisplay = append(domainsToDisplay, msg)
	}

	return domainsToDisplay
}
