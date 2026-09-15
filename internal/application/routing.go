package application

import (
	"math"
	"sort"
)

type loadItem struct {
	ID       string
	Kg, M3   float64
	CoordIdx int
	Lat, Lng float64
}

type vehicleCap struct {
	ID         string
	Kg, M3     float64
	Name, Code string
}

type trip struct {
	Vehicle vehicleCap
	Items   []loadItem
	Kg, M3  float64
}

func occupancy(kg, m3, capKg, capM3 float64) float64 {
	a, b := 0.0, 0.0
	if capKg > 0 {
		a = kg / capKg
	}
	if capM3 > 0 {
		b = m3 / capM3
	}
	if a > b {
		return a * 100
	}
	return b * 100
}

func packTrips(items []loadItem, vehicles []vehicleCap, depotLat, depotLng float64) (trips []trip, leftover []loadItem) {
	if len(vehicles) > 1 && len(items) > 1 {
		return packBySweep(items, vehicles, depotLat, depotLng)
	}
	return packFirstFit(items, vehicles)
}

func packFirstFit(items []loadItem, vehicles []vehicleCap) (trips []trip, leftover []loadItem) {
	sorted := append([]loadItem(nil), items...)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if loadRatio(sorted[j], vehicles) > loadRatio(sorted[i], vehicles) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	nextVeh := 0
	for _, it := range sorted {
		placed := false
		for i := range trips {
			if fits(trips[i], it) {
				trips[i].Items = append(trips[i].Items, it)
				trips[i].Kg += it.Kg
				trips[i].M3 += it.M3
				placed = true
				break
			}
		}
		if placed {
			continue
		}
		start := nextVeh
		opened := false
		for n := 0; n < len(vehicles); n++ {
			v := vehicles[(start+n)%len(vehicles)]
			t := trip{Vehicle: v}
			if fits(t, it) {
				t.Items = append(t.Items, it)
				t.Kg, t.M3 = it.Kg, it.M3
				trips = append(trips, t)
				nextVeh = (start + n + 1) % len(vehicles)
				opened = true
				break
			}
		}
		if !opened {
			leftover = append(leftover, it)
		}
	}
	return trips, leftover
}

func packBySweep(items []loadItem, vehicles []vehicleCap, lat0, lng0 float64) (trips []trip, leftover []loadItem) {
	sorted := sweepSort(items, lat0, lng0)
	k := len(vehicles)
	if k > len(sorted) {
		k = len(sorted)
	}
	n := len(sorted)
	idx := 0
	for i := 0; i < k; i++ {
		size := n / k
		if i < n%k {
			size++
		}
		group := sorted[idx : idx+size]
		idx += size
		v := vehicles[i]
		t := trip{Vehicle: v}
		for _, it := range group {
			if fits(t, it) {
				t.Items = append(t.Items, it)
				t.Kg += it.Kg
				t.M3 += it.M3
				continue
			}
			if len(t.Items) > 0 {
				trips = append(trips, t)
				t = trip{Vehicle: v}
			}
			if fits(t, it) {
				t.Items = append(t.Items, it)
				t.Kg, t.M3 = it.Kg, it.M3
			} else {
				leftover = append(leftover, it)
			}
		}
		if len(t.Items) > 0 {
			trips = append(trips, t)
		}
	}
	return trips, leftover
}

func sweepSort(items []loadItem, lat0, lng0 float64) []loadItem {
	type pair struct {
		it loadItem
		a  float64
	}
	ps := make([]pair, len(items))
	for i, it := range items {
		ps[i] = pair{it, math.Atan2(it.Lat-lat0, it.Lng-lng0)}
	}
	sort.Slice(ps, func(i, j int) bool { return ps[i].a < ps[j].a })
	out := make([]loadItem, len(ps))
	for i, p := range ps {
		out[i] = p.it
	}
	return out
}

func loadRatio(it loadItem, vehicles []vehicleCap) float64 {
	best := 0.0
	for _, v := range vehicles {
		r := occupancy(it.Kg, it.M3, v.Kg, v.M3) / 100
		if r > best {
			best = r
		}
	}
	return best
}

func fits(t trip, it loadItem) bool {
	return t.Kg+it.Kg <= t.Vehicle.Kg+1e-9 && t.M3+it.M3 <= t.Vehicle.M3+1e-9
}

func nearestNeighbor(dur [][]float64, stops []int) []int {
	if len(stops) <= 1 {
		return append([]int(nil), stops...)
	}
	return nnStarting(dur, stops, -1)
}

func nnStarting(cost [][]float64, stops []int, start int) []int {
	left := map[int]struct{}{}
	for _, s := range stops {
		left[s] = struct{}{}
	}
	out := make([]int, 0, len(stops))
	cur := 0
	if start >= 0 {
		if _, ok := left[start]; ok {
			out = append(out, start)
			delete(left, start)
			cur = start
		}
	}
	for len(left) > 0 {
		best, bestD := -1, 1e18
		for s := range left {
			d := matrixAt(cost, cur, s)
			if d < bestD {
				best, bestD = s, d
			}
		}
		out = append(out, best)
		delete(left, best)
		cur = best
	}
	return out
}

func farthestNeighbor(cost [][]float64, stops []int) []int {
	if len(stops) == 0 {
		return nil
	}
	first, best := stops[0], -1.0
	for _, s := range stops {
		d := matrixAt(cost, 0, s)
		if d > best {
			best, first = d, s
		}
	}
	return nnStarting(cost, stops, first)
}

func bestTour(cost [][]float64, stops []int) []int {
	if len(stops) == 0 {
		return nil
	}
	if len(stops) <= 12 {
		return heldKarp(cost, stops)
	}
	cands := [][]int{
		twoOpt(cost, nearestNeighbor(cost, stops)),
		twoOpt(cost, farthestNeighbor(cost, stops)),
	}
	best := cands[0]
	bestC := tourCost(cost, best)
	for _, c := range cands[1:] {
		if cc := tourCost(cost, c); cc < bestC {
			best, bestC = c, cc
		}
	}
	return best
}

func heldKarp(cost [][]float64, stops []int) []int {
	n := len(stops)
	if n <= 1 {
		return append([]int(nil), stops...)
	}
	const inf = 1e18
	N := 1 << n
	dp := make([][]float64, N)
	parent := make([][]int, N)
	for m := 0; m < N; m++ {
		dp[m] = make([]float64, n)
		parent[m] = make([]int, n)
		for i := 0; i < n; i++ {
			dp[m][i] = inf
			parent[m][i] = -1
		}
	}
	for i := 0; i < n; i++ {
		dp[1<<i][i] = matrixAt(cost, 0, stops[i])
	}
	for mask := 1; mask < N; mask++ {
		for last := 0; last < n; last++ {
			if mask&(1<<last) == 0 || dp[mask][last] >= inf {
				continue
			}
			for next := 0; next < n; next++ {
				if mask&(1<<next) != 0 {
					continue
				}
				nm := mask | (1 << next)
				cand := dp[mask][last] + matrixAt(cost, stops[last], stops[next])
				if cand < dp[nm][next] {
					dp[nm][next] = cand
					parent[nm][next] = last
				}
			}
		}
	}
	full := N - 1
	bestEnd, bestC := 0, inf
	for i := 0; i < n; i++ {
		cand := dp[full][i] + matrixAt(cost, stops[i], 0)
		if cand < bestC {
			bestC, bestEnd = cand, i
		}
	}
	order := make([]int, n)
	mask, cur := full, bestEnd
	for i := n - 1; i >= 0; i-- {
		order[i] = stops[cur]
		prev := parent[mask][cur]
		mask ^= 1 << cur
		cur = prev
	}
	return order
}

func uniqueTours(dur, dist [][]float64, stops []int) [][]int {
	seen := map[string]bool{}
	out := [][]int{}
	add := func(r []int) {
		if len(r) != len(stops) {
			return
		}
		k := tourKey(r)
		if seen[k] || seen[tourKey(reverseRoute(r))] {
			return
		}
		seen[k] = true
		out = append(out, r)
	}
	add(bestTour(dur, stops))
	add(bestTour(dist, stops))
	add(twoOpt(dur, farthestNeighbor(dur, stops)))
	for i := 0; i < len(stops) && len(out) < 3; i++ {
		add(twoOpt(dur, nnStarting(dur, stops, stops[i])))
	}
	if len(out) > 3 {
		out = out[:3]
	}
	return out
}

func tourKey(r []int) string {
	b := make([]byte, 0, len(r)*4)
	for _, x := range r {
		b = append(b, byte(x>>8), byte(x))
	}
	return string(b)
}

func reverseRoute(r []int) []int {
	out := append([]int(nil), r...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

func twoOpt(dur [][]float64, route []int) []int {
	if len(route) < 2 {
		return route
	}
	best := append([]int(nil), route...)
	improved := true
	for improved {
		improved = false
		for i := 0; i < len(best)-1; i++ {
			for k := i + 1; k < len(best); k++ {
				cand := reverseSeg(best, i, k)
				if tourCost(dur, cand) < tourCost(dur, best)-1e-9 {
					best = cand
					improved = true
				}
			}
		}
	}
	return best
}

func reverseSeg(route []int, i, k int) []int {
	out := append([]int(nil), route...)
	for i < k {
		out[i], out[k] = out[k], out[i]
		i++
		k--
	}
	return out
}

func tourCost(dur [][]float64, route []int) float64 {
	if len(route) == 0 {
		return 0
	}
	sum := matrixAt(dur, 0, route[0])
	for i := 0; i < len(route)-1; i++ {
		sum += matrixAt(dur, route[i], route[i+1])
	}
	sum += matrixAt(dur, route[len(route)-1], 0)
	return sum
}

func matrixAt(m [][]float64, i, j int) float64 {
	if i < 0 || j < 0 || i >= len(m) || j >= len(m[i]) {
		return 1e18
	}
	return m[i][j]
}
