package main

import "testing"

func TestSliderValue(t *testing.T) {
	const w = 900
	for _, tt := range []struct {
		x    float32
		want int
	}{{0, 1}, {w, 10}, {100, 2}, {400, 5}, {449, 5}, {451, 6}, {-50, 1}, {w + 50, 10}} {
		if got := sliderValue(tt.x, w); got != tt.want {
			t.Errorf("sliderValue(%v) = %d, want %d", tt.x, got, tt.want)
		}
	}
	for v := 1; v <= 10; v++ {
		if got := sliderValue(sliderX(v, w), w); got != v {
			t.Errorf("sliderValue(sliderX(%d)) = %d", v, got)
		}
	}
}

func TestThumbs(t *testing.T) {
	for _, tt := range []struct {
		lo, hi, v int
		want      thumb
	}{
		{3, 7, 1, lowThumb}, {3, 7, 4, lowThumb}, {3, 7, 6, highThumb}, {3, 7, 9, highThumb},
		{3, 7, 5, lowThumb}, // halfway: the low one
		{5, 5, 2, lowThumb}, {5, 5, 8, highThumb},
		{5, 5, 5, noThumb}, // on both: decided by where it's dragged
	} {
		if got := grab(tt.lo, tt.hi, tt.v); got != tt.want {
			t.Errorf("grab(%d, %d, %d) = %v, want %v", tt.lo, tt.hi, tt.v, got, tt.want)
		}
	}
	for _, tt := range []struct {
		lo, hi   int
		th       thumb
		v        int
		wlo, whi int
		wth      thumb
	}{
		{3, 7, lowThumb, 1, 1, 7, lowThumb},
		{3, 7, lowThumb, 9, 7, 7, lowThumb}, // can't pass the other
		{3, 7, highThumb, 2, 3, 3, highThumb},
		{3, 7, highThumb, 10, 3, 10, highThumb},
		{5, 5, noThumb, 5, 5, 5, noThumb},
		{5, 5, noThumb, 3, 3, 5, lowThumb},
		{5, 5, noThumb, 8, 5, 8, highThumb},
	} {
		lo, hi, th := moveThumb(tt.lo, tt.hi, tt.th, tt.v)
		if lo != tt.wlo || hi != tt.whi || th != tt.wth {
			t.Errorf("moveThumb(%d, %d, %v, %d) = %d, %d, %v; want %d, %d, %v", tt.lo, tt.hi, tt.th, tt.v, lo, hi, th, tt.wlo, tt.whi, tt.wth)
		}
	}
}
