// Package printer renders POS receipts as ESC/POS byte streams and dispatches
// them to a network thermal printer over raw TCP.
//
// Only the subset of ESC/POS commands needed for a basic receipt is wired up;
// most network thermal printers (Epson, Star, generic Chinese 58/80mm) accept
// this dialect.
package printer

import "bytes"

// ESC/POS control bytes.
const (
	esc = 0x1B
	gs  = 0x1D
	lf  = 0x0A
)

// Builder accumulates ESC/POS bytes with convenience helpers.
type Builder struct{ buf bytes.Buffer }

func NewBuilder() *Builder {
	b := &Builder{}
	b.Init()
	return b
}

func (b *Builder) Bytes() []byte { return b.buf.Bytes() }

// Init resets the printer (clears formatting, line spacing, etc.).
func (b *Builder) Init() *Builder {
	b.buf.WriteByte(esc)
	b.buf.WriteByte('@')
	return b
}

// AlignLeft / AlignCenter / AlignRight set the justification for subsequent text.
func (b *Builder) AlignLeft() *Builder   { return b.align(0) }
func (b *Builder) AlignCenter() *Builder { return b.align(1) }
func (b *Builder) AlignRight() *Builder  { return b.align(2) }

func (b *Builder) align(n byte) *Builder {
	b.buf.WriteByte(esc)
	b.buf.WriteByte('a')
	b.buf.WriteByte(n)
	return b
}

// Bold toggles bold text.
func (b *Builder) Bold(on bool) *Builder {
	b.buf.WriteByte(esc)
	b.buf.WriteByte('E')
	if on {
		b.buf.WriteByte(1)
	} else {
		b.buf.WriteByte(0)
	}
	return b
}

// DoubleSize prints subsequent text at 2x width and height. Pair with
// NormalSize() to switch back.
func (b *Builder) DoubleSize() *Builder { return b.size(0x11) }
func (b *Builder) NormalSize() *Builder { return b.size(0x00) }

func (b *Builder) size(n byte) *Builder {
	b.buf.WriteByte(gs)
	b.buf.WriteByte('!')
	b.buf.WriteByte(n)
	return b
}

// Text writes a raw string. No automatic line break.
func (b *Builder) Text(s string) *Builder {
	b.buf.WriteString(s)
	return b
}

// Line writes a string followed by a line feed.
func (b *Builder) Line(s string) *Builder {
	b.buf.WriteString(s)
	b.buf.WriteByte(lf)
	return b
}

// Feed advances `n` lines.
func (b *Builder) Feed(n int) *Builder {
	for i := 0; i < n; i++ {
		b.buf.WriteByte(lf)
	}
	return b
}

// Cut sends a partial-cut command. Many low-end printers also accept this as a
// no-op when they lack a cutter (their head just stays where it is).
func (b *Builder) Cut() *Builder {
	b.buf.WriteByte(gs)
	b.buf.WriteByte('V')
	b.buf.WriteByte(1)
	return b
}

// OpenDrawer fires the cash-drawer kick pulse on pin 2.
func (b *Builder) OpenDrawer() *Builder {
	// ESC p m t1 t2: m=0 (pin 2), t1=t2=50 (~100ms on, 100ms off).
	b.buf.WriteByte(esc)
	b.buf.WriteByte('p')
	b.buf.WriteByte(0)
	b.buf.WriteByte(50)
	b.buf.WriteByte(50)
	return b
}
