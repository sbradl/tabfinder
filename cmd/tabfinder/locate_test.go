package main

import (
	"testing"
	"time"
)

// The probing helpers find what the existing tests hard-code.
func TestLocate(t *testing.T) {
	h := newHarness(t)
	start := time.Now()
	r := h.fieldRect(&h.u.artist)
	t.Logf("artist field %v in %v", r, time.Since(start))
	if c := center(r); c.X < 100 || c.X > 330 || c.Y < 90 || c.Y > 120 {
		t.Errorf("artist field at %v", r)
	}
	if r.Dy() != 56 {
		t.Errorf("field height %d, want 56", r.Dy())
	}
	row := h.rowRect(0)
	t.Logf("row 0 %v", row)
	if row.Empty() || row.Dx() < 900 {
		t.Errorf("row 0 = %v", row)
	}
}
