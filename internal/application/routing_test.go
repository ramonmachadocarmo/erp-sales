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

func TestPackByDateNeverMixesDates(t *testing.T) {
	vehicles := []vehicleCap{{ID: "v1", Kg: 1000, M3: 10, Name: "Van", Code: "V1"}}
	items := []loadItem{
		{ID: "a", Kg: 10, M3: 0.1, CoordIdx: 1, Lat: -23.5, Lng: -46.6, Date: "2026-09-23"},
		{ID: "b", Kg: 10, M3: 0.1, CoordIdx: 2, Lat: -23.6, Lng: -46.7, Date: "2026-09-22"},
		{ID: "c", Kg: 10, M3: 0.1, CoordIdx: 3, Lat: -23.7, Lng: -46.8, Date: ""},
		{ID: "d", Kg: 10, M3: 0.1, CoordIdx: 4, Lat: -23.55, Lng: -46.65, Date: "2026-09-22"},
	}
	trips, left := packByDate(items, vehicles, -23.5, -46.6)
	if len(left) != 0 {
		t.Fatalf("leftover: %+v", left)
	}
	if len(trips) != 3 {
		t.Fatalf("want one trip per date (3), got %d", len(trips))
	}
	// oldest date first, undated last
	if trips[0].date != "2026-09-22" || trips[1].date != "2026-09-23" || trips[2].date != "" {
		t.Fatalf("order: %q %q %q", trips[0].date, trips[1].date, trips[2].date)
	}
	for _, tr := range trips {
		for _, it := range tr.Items {
			if it.Date != tr.date {
				t.Fatalf("trip %q carries an order due %q", tr.date, it.Date)
			}
		}
	}
	if len(trips[0].Items) != 2 {
		t.Fatalf("both 22/09 orders share a trip, got %d", len(trips[0].Items))
	}
}
