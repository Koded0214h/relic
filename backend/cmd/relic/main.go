package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Koded0214h/relic/backend/internal/config"
	"github.com/Koded0214h/relic/backend/internal/server"
)

func main() {
	if err:= run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil { return err }

	srv := &http.Server{
		Addr:		fmt.Sprintf(":%d", cfg.Port),
		Handler:    server.New(cfg).Router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout: 60 * time.Second,
	}

	go func() {
		log.Printf("relic listening on %d (%s)", cfg.Port, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	} ()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shtting down")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}
