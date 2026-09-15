package application

import "testing"

func TestPackTripsSplitsByCapacity(t *testing.T) {
	items := []loadItem{
		{ID: "a", Kg: 80, M3: 1},
		{ID: "b", Kg: 80, M3: 1},
		{ID: "c", Kg: 10, M3: 0.1},
	}
	vehicles := []vehicleCap{{ID: "v1", Kg: 100, M3: 2}}
	trips, leftover := packTrips(items, vehicles, 0, 0)
	if len(leftover) != 0 {
		t.Fatalf("leftover %+v", leftover)
	}
	if len(trips) != 2 {
		t.Fatalf("trips %d", len(trips))
	}
}

func TestPackTripsSpreadsAcrossVehicles(t *testing.T) {
	items := []loadItem{
		{ID: "a", Kg: 10, M3: 0.1, Lat: 0, Lng: 1},
		{ID: "b", Kg: 10, M3: 0.1, Lat: 0, Lng: -1},
		{ID: "c", Kg: 10, M3: 0.1, Lat: 1, Lng: 0},
		{ID: "d", Kg: 10, M3: 0.1, Lat: -1, Lng: 0},
	}
	vehicles := []vehicleCap{
		{ID: "v1", Kg: 100, M3: 10},
		{ID: "v2", Kg: 100, M3: 10},
	}
	trips, leftover := packTrips(items, vehicles, 0, 0)
	if len(leftover) != 0 {
		t.Fatalf("leftover %+v", leftover)
	}
	if len(trips) != 2 {
		t.Fatalf("trips %d", len(trips))
	}
	if len(trips[0].Items) < 1 || len(trips[1].Items) < 1 {
		t.Fatalf("empty trip %+v", trips)
	}
}

func TestUniqueToursCapsAtThree(t *testing.T) {
	dur := [][]float64{
		{0, 10, 30, 40},
		{10, 0, 5, 50},
		{30, 5, 0, 8},
		{40, 50, 8, 0},
	}
	dist := [][]float64{
		{0, 2, 9, 4},
		{2, 0, 7, 3},
		{9, 7, 0, 6},
		{4, 3, 6, 0},
	}
	got := uniqueTours(dur, dist, []int{1, 2, 3})
	if len(got) == 0 || len(got) > 3 {
		t.Fatalf("%d tours", len(got))
	}
}

func TestNearestNeighborAndTwoOpt(t *testing.T) {
	dur := [][]float64{
		{0, 10, 30, 40},
		{10, 0, 5, 50},
		{30, 5, 0, 8},
		{40, 50, 8, 0},
	}
	got := nearestNeighbor(dur, []int{1, 2, 3})
	if len(got) != 3 || got[0] != 1 {
		t.Fatalf("%v", got)
	}
	opt := twoOpt(dur, []int{1, 3, 2})
	if tourCost(dur, opt) > tourCost(dur, []int{1, 2, 3})+1e-9 {
		t.Fatalf("opt worse %v", opt)
	}
}

func TestHeldKarpBeatsNaive(t *testing.T) {
	dur := [][]float64{
		{0, 10, 30, 40},
		{10, 0, 5, 50},
		{30, 5, 0, 8},
		{40, 50, 8, 0},
	}
	got := heldKarp(dur, []int{1, 2, 3})
	if tourCost(dur, got) > tourCost(dur, []int{1, 2, 3})+1e-9 {
		t.Fatalf("%v cost %v", got, tourCost(dur, got))
	}
}
