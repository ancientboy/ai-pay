package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	api "ai-pay-backend/internal/http"
	"ai-pay-backend/internal/ops"
	"ai-pay-backend/internal/service"
	"ai-pay-backend/internal/storage"
)

func main() {
	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	var paymentSvc service.PaymentService
	var readyCheck func(context.Context) error
	var persistentStore *storage.Store
	mysqlDSN := os.Getenv("MYSQL_DSN")
	redisAddr := os.Getenv("REDIS_ADDR")
	if mysqlDSN != "" && redisAddr != "" {
		store, err := storage.New(appCtx, storage.Config{
			MySQLDSN:      mysqlDSN,
			RedisAddr:     redisAddr,
			RedisPassword: os.Getenv("REDIS_PASSWORD"),
		})
		if err != nil {
			log.Fatalf("init persistent storage failed: %v", err)
		}
		defer store.Close()
		persistentStore = store
		log.Printf("using persistent storage (mysql + redis)")
		paymentSvc = service.NewPersistent(store)
		readyCheck = store.Ping
		startOpsJobs(appCtx, store)
	} else {
		log.Printf("using in-memory storage (set MYSQL_DSN and REDIS_ADDR for persistence)")
		paymentSvc = service.New()
	}
	server := api.NewServerWithReadiness(paymentSvc, readyCheck)
	if persistentStore != nil {
		server.EnableRedisRateLimiters(persistentStore.Redis, 120, 60, time.Minute)
	}
	server.SetAdminBearerToken(os.Getenv("ADMIN_BEARER_TOKEN"))
	server.SetReadonlyBearerToken(os.Getenv("READONLY_BEARER_TOKEN"))
	server.SetCallbackToken(os.Getenv("CALLBACK_TOKEN"))
	server.SetCallbackSigningSecret(os.Getenv("CALLBACK_SIGNING_SECRET"))
	httpServer := &http.Server{
		Addr:              ":" + port,
		Handler:           server.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("ai-pay backend listening on :%s", port)
	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-appCtx.Done():
		log.Printf("shutdown signal received, stopping server...")
	case err := <-errCh:
		log.Fatalf("server failed: %v", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}
	log.Printf("server stopped gracefully")
}

func startOpsJobs(ctx context.Context, store *storage.Store) {
	jobs := ops.NewJobs(store)
	jobs.SetWebhookSigningSecret(os.Getenv("WEBHOOK_SIGNING_SECRET"))

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := jobs.RunStatusCompensation(context.Background()); err != nil {
					log.Printf("[ALERT] compensation job failed: %v", err)
				}
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				result, err := jobs.RunDailyReconcile(context.Background(), time.Now().UTC())
				if err != nil {
					log.Printf("[ALERT] reconcile job failed: %v", err)
					continue
				}
				log.Printf("reconcile checked=%d mismatched=%d", result.CheckedAccounts, result.Mismatched)
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				result, err := jobs.RunWebhookDelivery(context.Background(), time.Now().UTC(), 100)
				if err != nil {
					log.Printf("[ALERT] webhook delivery job failed: %v", err)
					continue
				}
				if result.Checked > 0 {
					log.Printf("webhook delivery checked=%d sent=%d retried=%d dead=%d skipped=%d",
						result.Checked, result.Sent, result.Retried, result.Dead, result.Skipped)
				}
			}
		}
	}()
}
