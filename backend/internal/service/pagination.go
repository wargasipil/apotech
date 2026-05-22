package service

// normPage sanitizes request pagination params into (limit, offset).
// Default page size 25, hard cap 200, non-negative offset. Used by every
// List* handler so paging behaves uniformly across the app.
func normPage(limit, offset int32) (int, int) {
	l := int(limit)
	if l <= 0 || l > 200 {
		l = 25
	}
	o := int(offset)
	if o < 0 {
		o = 0
	}
	return l, o
}
