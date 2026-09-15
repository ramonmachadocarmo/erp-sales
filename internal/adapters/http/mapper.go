package httpadapter

import (
	"erp-schema/model"
	"erp/services/sales-service/internal/domain"
)

func toDomainAddress(m model.Address) domain.OrderAddress {
	addr := domain.OrderAddress{
		Alias:      m.Alias,
		Zip:        m.Zip,
		Street:     m.Street,
		Number:     m.Number,
		Complement: m.Complement,
		District:   m.District,
		City:       m.City,
		State:      m.State,
		Lat:        m.Lat,
		Lng:        m.Lng,
	}
	if m.ID != nil {
		addr.ID = *m.ID
	}
	return addr
}
