package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	api "ai-pay-backend/internal/http"
	"ai-pay-backend/internal/ops"
	"ai-pay-backend/internal/service"
	"ai-pay-backend/internal/storage"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	var paymentSvc service.PaymentService
	mysqlDSN := os.Getenv("MYSQL_DSN")
	redisAddr := os.Getenv("REDIS_ADDR")
	if mysqlDSN != "" && redisAddr != "" {
		store, err := storage.New(context.Background(), storage.Config{
			MySQLDSN:      mysqlDSN,
			RedisAddr:     redisAddr,
			RedisPassword: os.Getenv("REDIS_PASSWORD"),
		})
		if err != nil {
			log.Fatalf("init persistent storage failed: %v", err)
		}
		defer store.Close()
		log.Printf("using persistent storage (mysql + redis)")
		paymentSvc = service.NewPersistent(store)
		startOpsJobs(store)
	} else {
		log.Printf("using in-memory storage (set MYSQL_DSN and REDIS_ADDR for persistence)")
		paymentSvc = service.New()
	}
	server := api.NewServer(paymentSvc)

	log.Printf("ai-pay backend listening on :%s", port)
	if err := http.ListenAndServe(":"+port, server.Routes()); err != nil {
		log.Fatal(err)
	}
}

func startOpsJobs(store *storage.Store) {
	jobs := ops.NewJobs(store)

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if _, err := jobs.RunStatusCompensation(context.Background()); err != nil {
				log.Printf("[ALERT] compensation job failed: %v", err)
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			result, err := jobs.RunDailyReconcile(context.Background(), time.Now().UTC())
			if err != nil {
				log.Printf("[ALERT] reconcile job failed: %v", err)
				continue
			}
			log.Printf("reconcile checked=%d mismatched=%d", result.CheckedAccounts, result.Mismatched)
		}
	}()
}
