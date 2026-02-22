package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gopherai-resume/internal/bootstrap"
	httptransport "gopherai-resume/internal/transport/http"
)

func main() {
	ctx := context.Background()

	app, err := bootstrap.New(ctx)
	if err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}
	defer func() {
		if err := app.Close(); err != nil {
			log.Printf("close resources failed: %v", err)
		}
	}()

	router := httptransport.NewRouter(app)
	server := &http.Server{
		Addr:              app.Config.HTTPAddr(),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("server starting on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	waitForShutdown(server)
}

func waitForShutdown(server *http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown failed: %v", err)
	}
}
