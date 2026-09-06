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

	"github.com/Koded0214h/relic/backend/internal/api/archive"
	"github.com/Koded0214h/relic/backend/internal/api/files"
	"github.com/Koded0214h/relic/backend/internal/codec"
	"github.com/Koded0214h/relic/backend/internal/codec/generic"
	"github.com/Koded0214h/relic/backend/internal/codec/jpg"
	"github.com/Koded0214h/relic/backend/internal/codec/raw"
	"github.com/Koded0214h/relic/backend/internal/config"
	"github.com/Koded0214h/relic/backend/internal/db"
	"github.com/Koded0214h/relic/backend/internal/job"
	"github.com/Koded0214h/relic/backend/internal/server"
	"github.com/Koded0214h/relic/backend/internal/store"
	authapi "github.com/Koded0214h/relic/backend/internal/api/auth"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	database, err := db.Open(cfg.DataDir + "/relic.db")
	if err != nil {
		return err
	}
	defer database.Close()
	if err := db.Migrate(database); err != nil {
		return err
	}

	objStore, err := store.New(cfg.DataDir + "/objects")
	if err != nil {
		return err
	}
	reg := codec.NewRegistry(generic.New(), jpg.New(), raw.New())
	runner := job.NewRunner(objStore, reg)

	archive.Init(runner, database)
	files.Init(database, objStore, reg)
	authapi.Init(database, !cfg.Dev())

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           server.New(cfg).Router,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("relic listening on %d (%s)", cfg.Port, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}
