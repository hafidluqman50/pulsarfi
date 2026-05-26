package app

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/horizonlabs/pulsarfi-backend/src/config"
)

func Run() {
	router, cleanup, err := buildHandler()
	if err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}
	defer cleanup()

	addr := config.GetEnv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
	}

	go func() {
		log.Printf("pulsarfi-backend listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
