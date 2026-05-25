package service

import "strings"

// maxResolveIDs caps a single Resolve<Domain>(ids) lookup. The frontend resolves
// only the IDs on its current page, so this is comfortably above any real page.
const maxResolveIDs = 500

// dedupeIDs cleans a Resolve<Domain> request's ids: trims blanks, removes
// duplicates (preserving first-seen order), and caps the count at maxResolveIDs.
func dedupeIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
		if len(out) >= maxResolveIDs {
			break
		}
	}
	return out
}

// normPage sanitizes request pagination params into (limit, offset).
// Default page size 25, hard cap 1000, non-negative offset. Used by every
// List* handler so paging behaves uniformly across the app. The cap matches
// the frontend's ALL_LIMIT (1000) "preload everything" sentinel — large limits
// are CLAMPED to 1000 (not reset to the default), so `useAll*` name-map
// preloads actually receive the full list instead of just the first page.
func normPage(limit, offset int32) (int, int) {
	l := int(limit)
	if l <= 0 {
		l = 25
	} else if l > 1000 {
		l = 1000
	}
	o := int(offset)
	if o < 0 {
		o = 0
	}
	return l, o
}
