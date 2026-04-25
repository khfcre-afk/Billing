package promo

import "testing"

func TestErrContains(t *testing.T) {
	cases := []struct {
		s, sub string
		want   bool
	}{
		{"duplicate key value violates unique constraint", "duplicate key", true},
		{"ERROR: duplicate", "duplicate key", false},
		{"", "x", false},
	}
	for _, c := range cases {
		if got := errContains(c.s, c.sub); got != c.want {
			t.Errorf("errContains(%q,%q)=%v want %v", c.s, c.sub, got, c.want)
		}
	}
}
