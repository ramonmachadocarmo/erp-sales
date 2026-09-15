package rabbitadapter

import (
	"context"
	"encoding/json"
	"log"

	"erp/pkg/rabbit"
	"erp/services/sales-service/internal/application"
	"erp/services/sales-service/internal/domain"
)

func Consume(client *rabbit.Client, svc *application.Service) error {
	if err := client.Consume("sales.stock-reserved.queue", func(body []byte) error {
		var ev domain.OrderEvent
		if err := json.Unmarshal(body, &ev); err != nil {
			return err
		}
		if err := svc.OnStockReserved(context.Background(), ev); err != nil {
			log.Printf("stock reserved %s: %v", ev.OrderID, err)
			return err
		}
		return nil
	}); err != nil {
		return err
	}
	if err := client.EnsureTopology("invoicing.exchange", "sales.nfe-issued.queue", "invoicing.nfe.issued"); err != nil {
		return err
	}
	return client.Consume("sales.nfe-issued.queue", func(body []byte) error {
		var ev domain.InvoiceEvent
		if err := json.Unmarshal(body, &ev); err != nil {
			return err
		}
		if err := svc.OnNFeIssued(context.Background(), ev); err != nil {
			log.Printf("nfe issued %s: %v", ev.SalesOrderID, err)
			return err
		}
		return nil
	})
}
