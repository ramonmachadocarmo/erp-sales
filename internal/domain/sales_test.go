package domain

import "testing"

func TestPickedQty(t *testing.T) {
	picks := []Pick{
		{ProductID: "a", Quantity: 1},
		{ProductID: "b", Quantity: 2},
		{ProductID: "a", Quantity: 1.5},
	}
	if PickedQty(picks, "a") != 2.5 || PickedQty(picks, "b") != 2 || PickedQty(picks, "c") != 0 {
		t.Fatal(PickedQty(picks, "a"), PickedQty(picks, "b"), PickedQty(picks, "c"))
	}
}
