package application

import (
	"context"
	"fmt"
	"strings"

	"erp/services/sales-service/internal/domain"
)

func (s *Service) ListCandidates(ctx context.Context) ([]domain.DeliveryCandidate, error) {
	orders, err := s.orders.List(ctx, nil, nil)
	if err != nil {
		return nil, err
	}
	planned := map[string]struct{}{}
	if s.plans != nil {
		planned, err = s.plans.PlannedOrderIDs(ctx)
		if err != nil {
			return nil, err
		}
	}
	var out []domain.DeliveryCandidate
	for _, o := range orders {
		if o.Status != "PICKED" && o.Status != "UNDELIVERED" {
			continue
		}
		c := domain.DeliveryCandidate{Order: o, Planned: false}
		if _, ok := planned[o.ID]; ok {
			c.Planned = true
		}
		if lat, lng, ok := s.resolveGeo(ctx, o); ok {
			c.HasGeo = true
			c.Address.Lat = &lat
			c.Address.Lng = &lng
		}
		c.WeightKg, c.VolumeM3 = s.orderLoad(ctx, o)
		out = append(out, c)
	}
	if out == nil {
		out = []domain.DeliveryCandidate{}
	}
	return out, nil
}

func (s *Service) ListPlans(ctx context.Context) ([]domain.DeliveryPlan, error) {
	if s.plans == nil {
		return []domain.DeliveryPlan{}, nil
	}
	plans, err := s.plans.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range plans {
		s.enrichPlan(ctx, &plans[i])
	}
	return plans, nil
}

func (s *Service) GetPlan(ctx context.Context, id string) (domain.DeliveryPlan, error) {
	if s.plans == nil {
		return domain.DeliveryPlan{}, domain.ErrNotFound
	}
	p, err := s.plans.Get(ctx, id)
	if err != nil {
		return domain.DeliveryPlan{}, err
	}
	s.enrichPlan(ctx, &p)
	return p, nil
}

func (s *Service) ConfirmPlan(ctx context.Context, id string, opt domain.RouteOption) (domain.DeliveryPlan, error) {
	if s.plans == nil {
		return domain.DeliveryPlan{}, domain.ErrNotFound
	}
	p, err := s.plans.Get(ctx, id)
	if err != nil {
		return domain.DeliveryPlan{}, err
	}
	if p.Status != "PLANNED" && p.Status != "CONFIRMED" {
		return domain.DeliveryPlan{}, domain.ErrInvalid
	}
	if len(opt.Stops) > 0 {
		p.Stops = opt.Stops
		p.Geometry = opt.Geometry
		p.DistanceM = opt.DistanceM
		p.DurationS = opt.DurationS
		if err := s.plans.ReplaceRoute(ctx, p); err != nil {
			return domain.DeliveryPlan{}, err
		}
	}
	if err := s.plans.UpdateStatus(ctx, id, "CONFIRMED"); err != nil {
		return domain.DeliveryPlan{}, err
	}
	return s.GetPlan(ctx, id)
}

func (s *Service) CreatePlans(ctx context.Context, centerID string, vehicleIDs, orderIDs []string) (domain.PlanResult, error) {
	if s.plans == nil || s.router == nil || s.catalog == nil {
		return domain.PlanResult{}, domain.ErrInvalid
	}
	if strings.TrimSpace(centerID) == "" || len(vehicleIDs) == 0 || len(orderIDs) == 0 {
		return domain.PlanResult{}, domain.ErrInvalid
	}
	center, err := s.dir.GetCenter(ctx, centerID)
	if err != nil {
		return domain.PlanResult{}, err
	}
	var vehicles []vehicleCap
	for _, id := range vehicleIDs {
		v, err := s.dir.GetVehicle(ctx, id)
		if err != nil {
			return domain.PlanResult{}, err
		}
		if !v.Active || v.CapacityKg <= 0 || v.CapacityM3 <= 0 {
			continue
		}
		vehicles = append(vehicles, vehicleCap{ID: v.ID, Kg: v.CapacityKg, M3: v.CapacityM3, Name: v.Name, Code: v.Code})
	}
	if len(vehicles) == 0 {
		return domain.PlanResult{}, domain.ErrInvalid
	}
	if err := s.plans.ReleaseOrders(ctx, orderIDs); err != nil {
		return domain.PlanResult{}, err
	}
	planned, err := s.plans.PlannedOrderIDs(ctx)
	if err != nil {
		return domain.PlanResult{}, err
	}
	coords := []domain.Coord{{Lat: center.Lat, Lng: center.Lng}}
	var items []loadItem
	var skipped []domain.SkippedStop
	seen := map[string]struct{}{}
	for _, oid := range orderIDs {
		if _, dup := seen[oid]; dup {
			continue
		}
		seen[oid] = struct{}{}
		if _, ok := planned[oid]; ok {
			skipped = append(skipped, domain.SkippedStop{OrderID: oid, Reason: "already planned"})
			continue
		}
		o, err := s.orders.Get(ctx, oid)
		if err != nil {
			skipped = append(skipped, domain.SkippedStop{OrderID: oid, Reason: "not found"})
			continue
		}
		if o.Status != "PICKED" && o.Status != "UNDELIVERED" {
			skipped = append(skipped, domain.SkippedStop{OrderID: oid, Reason: "not picked"})
			continue
		}
		lat, lng, ok := s.resolveGeo(ctx, o)
		if !ok {
			skipped = append(skipped, domain.SkippedStop{OrderID: oid, Reason: "missing coordinates"})
			continue
		}
		kg, m3 := s.orderLoad(ctx, o)
		coords = append(coords, domain.Coord{Lat: lat, Lng: lng})
		items = append(items, loadItem{ID: o.ID, Kg: kg, M3: m3, CoordIdx: len(coords) - 1, Lat: lat, Lng: lng, Date: o.DeliveryDate})
	}
	if len(items) == 0 {
		reason := "nenhuma entrega com coordenada"
		if len(skipped) > 0 && skipped[0].Reason != "" {
			if skipped[0].Reason == "missing coordinates" {
				reason = "endereço sem coordenada"
			} else {
				reason = skipped[0].Reason
			}
		}
		return domain.PlanResult{Skipped: skipped}, fmt.Errorf("%s", reason)
	}
	trips, leftover := packByDate(items, vehicles, center.Lat, center.Lng)
	for _, it := range leftover {
		skipped = append(skipped, domain.SkippedStop{OrderID: it.ID, Reason: "exceeds vehicle capacity"})
	}
	if len(trips) == 0 {
		return domain.PlanResult{Skipped: skipped}, domain.ErrInvalid
	}
	durations, distances, err := s.router.Table(ctx, coords)
	if err != nil {
		return domain.PlanResult{}, err
	}
	var plans []domain.DeliveryPlan
	labels := []string{"Melhor tempo", "Menor distância", "Alternativa"}
	for _, t := range trips {
		stopIdx := make([]int, len(t.Items))
		byIdx := map[int]loadItem{}
		for i, it := range t.Items {
			stopIdx[i] = it.CoordIdx
			byIdx[it.CoordIdx] = it
		}
		tours := uniqueTours(durations, distances, stopIdx)
		if len(tours) == 0 {
			continue
		}
		var options []domain.RouteOption
		for i, order := range tours {
			stops, dist, dur := buildStops(order, byIdx, distances, durations)
			routeCoords := []domain.Coord{{Lat: center.Lat, Lng: center.Lng}}
			for _, idx := range order {
				routeCoords = append(routeCoords, coords[idx])
			}
			routeCoords = append(routeCoords, domain.Coord{Lat: center.Lat, Lng: center.Lng})
			geom, err := s.router.Geometry(ctx, routeCoords)
			if err != nil {
				if i == 0 {
					return domain.PlanResult{}, err
				}
				continue
			}
			label := labels[i]
			if i >= len(labels) {
				label = "Alternativa"
			}
			options = append(options, domain.RouteOption{
				Label: label, DistanceM: dist, DurationS: dur, Geometry: geom, Stops: stops, Selected: i == 0,
			})
		}
		if len(options) == 0 {
			continue
		}
		best := options[0]
		plans = append(plans, domain.DeliveryPlan{
			CenterID: center.ID, VehicleID: t.Vehicle.ID, DeliveryDate: t.date, Status: "PLANNED",
			DistanceM: best.DistanceM, DurationS: best.DurationS, WeightKg: t.Kg, VolumeM3: t.M3,
			OccupancyPct: occupancy(t.Kg, t.M3, t.Vehicle.Kg, t.Vehicle.M3),
			Geometry:     best.Geometry, Stops: best.Stops, Options: options,
			CenterName: center.Name, VehicleName: t.Vehicle.Name, VehicleCode: t.Vehicle.Code,
			CapacityKg: t.Vehicle.Kg, CapacityM3: t.Vehicle.M3,
		})
	}
	saved, err := s.plans.CreateMany(ctx, plans)
	if err != nil {
		return domain.PlanResult{}, err
	}
	for i := range saved {
		s.enrichPlan(ctx, &saved[i])
		if i < len(plans) {
			saved[i].Options = plans[i].Options
			for j := range saved[i].Options {
				for k := range saved[i].Options[j].Stops {
					id := saved[i].Options[j].Stops[k].SalesOrderID
					o, err := s.orders.Get(ctx, id)
					if err != nil {
						continue
					}
					saved[i].Options[j].Stops[k].CustomerID = o.CustomerID
					saved[i].Options[j].Stops[k].Address = o.Address
				}
			}
		}
	}
	return domain.PlanResult{Plans: saved, Skipped: skipped}, nil
}

func (s *Service) resolveGeo(ctx context.Context, o domain.Order) (float64, float64, bool) {
	if o.Address.Lat != nil && o.Address.Lng != nil && coordsInState(o.Address.State, *o.Address.Lat, *o.Address.Lng) && !nearCityCenter(o.Address.City, o.Address.State, *o.Address.Lat, *o.Address.Lng) {
		return *o.Address.Lat, *o.Address.Lng, true
	}
	if s.dir == nil {
		return 0, 0, false
	}
	lat, lng, err := s.dir.SearchAddress(ctx, o.Address)
	if err != nil || (lat == 0 && lng == 0) || !coordsInState(o.Address.State, lat, lng) {
		return 0, 0, false
	}
	if o.ID != "" {
		_ = s.orders.UpdateAddressGeo(ctx, o.ID, lat, lng)
	}
	return lat, lng, true
}

func nearCityCenter(city, uf string, lat, lng float64) bool {
	p, ok := cityCenter[foldCity(city)+"|"+strings.ToUpper(strings.TrimSpace(uf))]
	if !ok {
		return false
	}
	dlat, dlng := lat-p[0], lng-p[1]
	return dlat*dlat+dlng*dlng < 0.05*0.05
}

func foldCity(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	r := strings.NewReplacer("á", "a", "à", "a", "â", "a", "ã", "a", "é", "e", "ê", "e", "í", "i", "ó", "o", "ô", "o", "õ", "o", "ú", "u", "ç", "c")
	return r.Replace(s)
}

var cityCenter = map[string][2]float64{
	"rio branco|AC": {-9.974, -67.824}, "maceio|AL": {-9.666, -35.735}, "macapa|AP": {0.034, -51.069},
	"manaus|AM": {-3.119, -60.022}, "salvador|BA": {-12.971, -38.501}, "fortaleza|CE": {-3.732, -38.527},
	"brasilia|DF": {-15.794, -47.882}, "vitoria|ES": {-20.315, -40.312}, "goiania|GO": {-16.686, -49.264},
	"sao luis|MA": {-2.530, -44.307}, "cuiaba|MT": {-15.601, -56.097}, "campo grande|MS": {-20.469, -54.620},
	"belo horizonte|MG": {-19.917, -43.935}, "belem|PA": {-1.456, -48.504}, "joao pessoa|PB": {-7.115, -34.863},
	"curitiba|PR": {-25.429, -49.272}, "recife|PE": {-8.054, -34.881}, "teresina|PI": {-5.089, -42.802},
	"rio de janeiro|RJ": {-22.907, -43.173}, "natal|RN": {-5.794, -35.211}, "porto alegre|RS": {-30.034, -51.218},
	"porto velho|RO": {-8.761, -63.900}, "boa vista|RR": {2.823, -60.676}, "florianopolis|SC": {-27.595, -48.548},
	"sao paulo|SP": {-23.551, -46.633}, "aracaju|SE": {-10.909, -37.075}, "palmas|TO": {-10.184, -48.334},
}

func coordsInState(uf string, lat, lng float64) bool {
	uf = strings.ToUpper(strings.TrimSpace(uf))
	b, ok := ufBox[uf]
	if !ok {
		return lat >= -34 && lat <= 6 && lng >= -74 && lng <= -32
	}
	return lat >= b[0] && lat <= b[1] && lng >= b[2] && lng <= b[3]
}

var ufBox = map[string][4]float64{
	"AC": {-11.2, -7.1, -74.0, -66.6}, "AL": {-10.5, -8.8, -38.3, -35.1},
	"AP": {-1.3, 4.5, -54.9, -49.8}, "AM": {-11.2, 2.3, -73.9, -56.0},
	"BA": {-18.4, -8.5, -46.6, -37.3}, "CE": {-7.9, -2.8, -41.5, -37.2},
	"DF": {-16.1, -15.4, -48.3, -47.3}, "ES": {-21.3, -17.9, -41.9, -39.5},
	"GO": {-19.5, -12.4, -53.3, -45.9}, "MA": {-10.3, -1.0, -48.8, -41.8},
	"MT": {-18.1, -7.3, -61.7, -50.2}, "MS": {-24.1, -17.1, -58.2, -50.9},
	"MG": {-23.0, -14.2, -51.1, -39.8}, "PA": {-9.9, 2.6, -58.9, -46.0},
	"PB": {-8.4, -6.0, -38.9, -34.7}, "PR": {-26.8, -22.5, -54.7, -48.0},
	"PE": {-9.6, -7.1, -41.4, -34.8}, "PI": {-10.9, -2.7, -45.9, -40.3},
	"RJ": {-23.4, -20.7, -44.9, -40.9}, "RN": {-7.0, -4.8, -38.6, -34.9},
	"RS": {-33.8, -27.0, -57.7, -49.6}, "RO": {-13.7, -7.9, -66.9, -59.7},
	"RR": {-1.6, 5.3, -64.8, -58.8}, "SC": {-29.4, -25.9, -53.9, -48.3},
	"SP": {-25.4, -19.7, -53.2, -44.1}, "SE": {-11.6, -9.5, -38.3, -36.3},
	"TO": {-13.5, -5.1, -50.8, -45.6},
}

func (s *Service) orderLoad(ctx context.Context, o domain.Order) (kg, m3 float64) {
	if s.catalog == nil {
		return 0, 0
	}
	for _, it := range o.Items {
		p, err := s.catalog.Product(ctx, it.ProductID)
		if err != nil {
			continue
		}
		kg += it.Quantity * p.WeightKg
		m3 += it.Quantity * p.VolumeM3
	}
	return kg, m3
}

func (s *Service) enrichPlan(ctx context.Context, p *domain.DeliveryPlan) {
	if c, err := s.dir.GetCenter(ctx, p.CenterID); err == nil {
		p.CenterName = c.Name
		p.CenterLat = c.Lat
		p.CenterLng = c.Lng
	}
	if v, err := s.dir.GetVehicle(ctx, p.VehicleID); err == nil {
		p.VehicleName = v.Name
		p.VehicleCode = v.Code
		p.CapacityKg = v.CapacityKg
		p.CapacityM3 = v.CapacityM3
	}
	for i, st := range p.Stops {
		o, err := s.orders.Get(ctx, st.SalesOrderID)
		if err != nil {
			continue
		}
		st.CustomerID = o.CustomerID
		st.Address = o.Address
		if o.Address.Lat != nil {
			st.Lat = *o.Address.Lat
		}
		if o.Address.Lng != nil {
			st.Lng = *o.Address.Lng
		}
		st.WeightKg, st.VolumeM3 = s.orderLoad(ctx, o)
		p.Stops[i] = st
	}
}

func (s *Service) refreshPlanStatus(ctx context.Context, orderID string) {
	if s.plans == nil {
		return
	}
	p, err := s.plans.ByOrder(ctx, orderID)
	if err != nil {
		return
	}
	allDone, anyDone := true, false
	for _, st := range p.Stops {
		o, err := s.orders.Get(ctx, st.SalesOrderID)
		if err != nil {
			allDone = false
			continue
		}
		if o.Status == "DELIVERED" {
			anyDone = true
		} else {
			allDone = false
		}
	}
	status := p.Status
	if p.Status == "CANCELLED" {
		return
	}
	if allDone {
		status = "DONE"
	} else if anyDone {
		status = "IN_PROGRESS"
	} else if p.Status == "CONFIRMED" || p.Status == "IN_PROGRESS" {
		status = "CONFIRMED"
	} else {
		status = "PLANNED"
	}
	_ = s.plans.UpdateStatus(ctx, p.ID, status)
}

func buildStops(order []int, byIdx map[int]loadItem, distances, durations [][]float64) (stops []domain.PlanStop, dist, dur float64) {
	prev := 0
	for i, idx := range order {
		it := byIdx[idx]
		legD := matrixAt(distances, prev, idx)
		legT := matrixAt(durations, prev, idx)
		dist += legD
		dur += legT
		stops = append(stops, domain.PlanStop{
			Seq: i + 1, SalesOrderID: it.ID, DistanceM: legD, DurationS: legT,
			Lat: it.Lat, Lng: it.Lng, WeightKg: it.Kg, VolumeM3: it.M3,
		})
		prev = idx
	}
	dist += matrixAt(distances, prev, 0)
	dur += matrixAt(durations, prev, 0)
	return stops, dist, dur
}
