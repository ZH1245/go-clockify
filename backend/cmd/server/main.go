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

	"github.com/ZH1245/go-clockify/backend/internal/config"
	"github.com/ZH1245/go-clockify/backend/internal/database"
	"github.com/ZH1245/go-clockify/backend/internal/sweeper"
	"github.com/ZH1245/go-clockify/backend/models"
	"github.com/ZH1245/go-clockify/backend/routes"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}

	// Project first: Task has a foreign key to it.
	if err := database.Migrate(db, &models.Project{}, &models.Task{}, &models.TaskItem{}); err != nil {
		log.Fatalf("migrate database: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.PingContext(r.Context()) != nil {
			http.Error(w, `{"status":"database unreachable"}`, http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})
	routes.RegisterTaskRoutes(mux, db)
	routes.RegisterTaskItemRoutes(mux, db)
	routes.RegisterProjectRoutes(mux, db)
	routes.RegisterDashboardRoutes(mux, db)
	routes.RegisterTimerRoutes(mux, db)

	// The sweeper is a long-lived background goroutine, so it gets a context of its own
	// rather than a request context: it must outlive every request. stopSweeper cancels
	// that context on shutdown, which is what ends the goroutine.
	sweepCtx, stopSweeper := context.WithCancel(context.Background())
	sweeper.Start(sweepCtx, db, cfg.SweepInterval, cfg.TimerPingTimeout)

	server := &http.Server{Addr: ":" + cfg.Port, Handler: withCORS(cfg.FrontendOrigin, mux)}

	// The server runs in its own goroutine so main can wait for a shutdown signal below.
	go func() {
		log.Printf("listening on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	// Block until Ctrl+C or `docker compose stop` (SIGTERM) arrives on the channel.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	// Stop accepting new requests and give running ones up to 10 seconds to finish.
	log.Println("shutting down")
	stopSweeper() // background work with no request to finish, so it stops immediately
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.Close()
	}
}
