package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"erp/pkg/audit"
	"erp/pkg/config"
	"erp/pkg/httpserver"
	"erp/pkg/outbox"
	"erp/pkg/postgres"
	"erp/pkg/rabbit"
	cashclient "erp/services/sales-service/internal/adapters/cashflow"
	configclient "erp/services/sales-service/internal/adapters/config"
	httpadapter "erp/services/sales-service/internal/adapters/http"
	identityclient "erp/services/sales-service/internal/adapters/identity"
	osrmadapter "erp/services/sales-service/internal/adapters/osrm"
	pgadapter "erp/services/sales-service/internal/adapters/postgres"
	rabbitadapter "erp/services/sales-service/internal/adapters/rabbit"
	stockclient "erp/services/sales-service/internal/adapters/stock"
	"erp/services/sales-service/internal/application"
	"erp/services/sales-service/migrations"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := postgres.Connect(ctx, cfg.Postgres.DSN())
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := postgres.Migrate(ctx, pool, migrations.FS, "."); err != nil {
		log.Fatal(err)
	}

	bus, err := rabbit.Wait(cfg.Rabbit.URI(), 15)
	if err != nil {
		log.Fatal(err)
	}
	defer bus.Close()

	// Audit logging for the picking bypass action only (see Service.BypassPicking) — this is
	// NOT the gateway's automatic per-request audit trail (pkg/audit's own doc comment), a
	// deliberate one-off exception because a bypass is a business decision worth recording
	// on its own terms (who requested it, who approved it, why), not just "PUT this order".
	// Best-effort: a bypass must still work if Mongo is briefly unavailable, so a connect
	// failure here logs and continues with a nil logger (nil-safe, see pkg/audit.Logger.Log)
	// instead of crashing the whole service over an audit sink.
	auditLogger, err := audit.Connect(ctx, cfg.Mongo.URI(), cfg.Mongo.DB)
	if err != nil {
		log.Printf("audit log unavailable, continuing without it: %v", err)
		auditLogger = nil
	}

	repo := pgadapter.New(pool)
	svc := application.New(
		pgadapter.Orders{Repo: repo},
		configclient.New(cfg.ConfigBaseURL),
		cashclient.New(cfg.CashflowBaseURL),
		stockclient.New(cfg.StockBaseURL),
		osrmadapter.New(cfg.OSRMBaseURL),
		pgadapter.NewPlanRepo(pool),
		identityclient.New(cfg.IdentityBaseURL),
		auditLogger,
	)
	if err := rabbitadapter.Consume(bus, svc); err != nil {
		log.Fatal(err)
	}
	go outbox.Run(ctx, repo, bus, func(eventType string) (string, string) {
		return "sales.exchange", eventType
	}, 2*time.Second)

	engine := httpserver.New(cfg.ServiceName)
	httpadapter.New(svc).Register(engine, httpserver.JWT(cfg.JWTSecret, cfg.JWTIssuer))

	srv := &http.Server{Addr: ":" + cfg.HTTPPort, Handler: engine}
	go func() {
		log.Printf("%s listening on %s", cfg.ServiceName, srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	<-ctx.Done()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdown)
}
