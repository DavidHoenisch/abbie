package state

import "testing"

func TestPoolKey_stable(t *testing.T) {
	a := PoolKey([]string{"first", "second"})
	b := PoolKey([]string{"second", "first"})
	if a == b {
		t.Fatal("order should affect pool identity")
	}
	c := PoolKey([]string{"first", "second"})
	if a != c {
		t.Fatal("deterministic mismatch")
	}
}
