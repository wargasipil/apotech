package printer

import (
	"fmt"
	"strings"
	"time"
)

// Receipt is the rendering-ready view of a completed sale. The caller is
// expected to denormalize medicine names / cashier name / customer name
// before handing it in (the printer package does NOT touch the DB).
type Receipt struct {
	SaleNo      string
	CompletedAt time.Time
	Cashier     string
	Customer    string // optional; empty = walk-in
	Items       []ReceiptLine
	Subtotal    int64
	Total       int64
	Paid        int64
	Payment     string // "CASH" | "BPJS" | "INSURANCE_OTHER"
	Change      int64
}

type ReceiptLine struct {
	Qty       int32
	Name      string
	LineTotal int64
}

// Settings collects per-shop appearance config (header/footer text, paper
// width). Validate sets sane defaults so callers can pass a zero-value struct
// for testing.
type Settings struct {
	Width      int
	Header     []string
	Footer     []string
	OpenDrawer bool
}

func (s *Settings) normalize() {
	if s.Width <= 0 {
		s.Width = 32
	}
}

// Render produces a complete ESC/POS byte stream for the receipt.
func Render(r Receipt, s Settings) []byte {
	s.normalize()
	b := NewBuilder()

	// Header: centered, bold first line.
	b.AlignCenter()
	for i, line := range s.Header {
		if i == 0 {
			b.Bold(true).Line(line).Bold(false)
		} else {
			b.Line(line)
		}
	}
	if len(s.Header) > 0 {
		b.Line(rule(s.Width, '='))
	}

	// Meta block (left-aligned).
	b.AlignLeft()
	if r.SaleNo != "" {
		b.Line("No: " + r.SaleNo)
	}
	if !r.CompletedAt.IsZero() {
		b.Line(r.CompletedAt.Format("2006-01-02 15:04"))
	}
	if r.Cashier != "" {
		b.Line("Kasir: " + r.Cashier)
	}
	if r.Customer != "" {
		b.Line("Pelanggan: " + r.Customer)
	}
	b.Line(rule(s.Width, '-'))

	// Items.
	for _, it := range r.Items {
		title := fmt.Sprintf("%d x %s", it.Qty, it.Name)
		amount := formatIDR(it.LineTotal)
		b.Line(twoCol(s.Width, title, amount))
	}
	b.Line(rule(s.Width, '-'))

	// Totals.
	b.Line(twoCol(s.Width, "Subtotal", formatIDR(r.Subtotal)))
	b.Bold(true).Line(twoCol(s.Width, "TOTAL", formatIDR(r.Total))).Bold(false)
	payLabel := paymentLabel(r.Payment)
	b.Line(twoCol(s.Width, "Bayar ("+payLabel+")", formatIDR(r.Paid)))
	if r.Payment == "CASH" {
		b.Line(twoCol(s.Width, "Kembali", formatIDR(r.Change)))
	}
	b.Line(rule(s.Width, '='))

	// Footer.
	b.AlignCenter()
	for _, line := range s.Footer {
		b.Line(line)
	}

	// Tail: blank lines so the cut/tear is clear of content, then cut, then
	// optional drawer kick.
	b.Feed(4)
	b.Cut()
	if s.OpenDrawer {
		b.OpenDrawer()
	}
	return b.Bytes()
}

// rule returns a string of `width` copies of `ch`.
func rule(width int, ch byte) string {
	if width <= 0 {
		return ""
	}
	return strings.Repeat(string(ch), width)
}

// twoCol formats a left-text + right-text line padded to width. If left is too
// long to fit alongside right, it is printed on its own line and right is
// pushed to the next line, right-aligned.
func twoCol(width int, left, right string) string {
	free := width - len(right)
	if free >= len(left)+1 {
		return left + strings.Repeat(" ", free-len(left)) + right
	}
	// Wrap: left on row 1, right on row 2 right-aligned.
	pad := width - len(right)
	if pad < 0 {
		pad = 0
	}
	return left + "\n" + strings.Repeat(" ", pad) + right
}

// formatIDR renders an integer minor-unit amount as "Rp 12.345" (Indonesian
// thousands separator). Negative numbers are printed with a leading minus.
func formatIDR(n int64) string {
	neg := ""
	if n < 0 {
		neg = "-"
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	// Insert dots every 3 digits from the right.
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	return "Rp " + neg + b.String()
}

func paymentLabel(p string) string {
	switch p {
	case "CASH":
		return "Tunai"
	case "BPJS":
		return "BPJS"
	case "INSURANCE_OTHER":
		return "Asuransi"
	default:
		return "?"
	}
}
