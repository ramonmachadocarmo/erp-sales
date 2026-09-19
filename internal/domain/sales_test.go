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

func TestPickRequirementsExpandsKits(t *testing.T) {
	recipes := map[string][]OrderItemComponent{"kit": {{ProductID: "apple", Quantity: 2}, {ProductID: "bag", Quantity: 1}}}
	items := []OrderItem{{ProductID: "kit", Quantity: 3}, {ProductID: "plain", Quantity: 1}}

	req := PickRequirements(items, nil, recipes)
	if req["apple"] != 6 || req["bag"] != 3 || req["plain"] != 1 || req["kit"] != 3 {
		t.Fatal(req)
	}

	subst := []OrderItem{{ProductID: "kit", Quantity: 1, Components: []OrderItemComponent{{ProductID: "pear", Quantity: 4}}}}
	if r := PickRequirements(subst, nil, recipes); r["pear"] != 4 || r["apple"] != 0 || r["kit"] != 1 {
		t.Fatal(r)
	}

	legacy := []Pick{{ProductID: "kit", Quantity: 3}}
	if r := PickRequirements(items, legacy, recipes); r["apple"] != 0 || r["plain"] != 1 {
		t.Fatal(r)
	}
}

func TestKitComponentsPicked(t *testing.T) {
	recipes := map[string][]OrderItemComponent{"kit": {{ProductID: "apple", Quantity: 2}}}
	items := []OrderItem{{ProductID: "kit", Quantity: 2}}
	if KitComponentsPicked(items, []Pick{{ProductID: "apple", Quantity: 3}}, recipes, "kit") {
		t.Fatal("components incomplete")
	}
	if !KitComponentsPicked(items, []Pick{{ProductID: "apple", Quantity: 4}}, recipes, "kit") {
		t.Fatal("components complete")
	}
}
