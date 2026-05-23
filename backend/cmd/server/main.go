package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/horizonlabs/pulsarfi-backend/internal/bridge"
	"github.com/horizonlabs/pulsarfi-backend/internal/oracle"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rpcURL := getEnv("ETH_RPC_URL", "https://sepolia-rollup.arbitrum.io/rpc")

	ethClient, err := bridge.NewEthClient(ctx, rpcURL)
	if err != nil {
		log.Fatalf("failed to connect to Ethereum node: %v", err)
	}
	defer ethClient.Close()

	log.Printf("connected to Arbitrum Sepolia at %s", rpcURL)

	oracleSvc := oracle.New()

	mux := http.NewServeMux()
	mux.HandleFunc("/health",  handleHealth)
	mux.HandleFunc("/prices",  oracleSvc.HandlePrices)
	mux.HandleFunc("/mint",    bridge.HandleMint(ethClient))
	mux.HandleFunc("/burn",    bridge.HandleBurn(ethClient))
	mux.HandleFunc("/reserves", bridge.HandleReserves(ethClient))

	srv := &http.Server{
		Addr:         getEnv("LISTEN_ADDR", ":8080"),
		Handler:      corsMiddleware(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Printf("horizon-bridge listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown error: %v", err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok","version":"0.4.1","network":"arbitrum-sepolia"}`))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
