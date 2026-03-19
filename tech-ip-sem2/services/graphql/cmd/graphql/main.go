package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/sun1tar/MIREA-TIP-Practice-32/tech-ip-sem2/graphql/graph"
	"github.com/sun1tar/MIREA-TIP-Practice-32/tech-ip-sem2/graphql/internal/config"
	"github.com/sun1tar/MIREA-TIP-Practice-32/tech-ip-sem2/graphql/internal/logger"
	"github.com/sun1tar/MIREA-TIP-Practice-32/tech-ip-sem2/graphql/internal/repository"
)

// HealthResponse структура для ответа health check
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Service   string    `json:"service"`
}

// healthHandler возвращает 200 OK для Kubernetes probes
func healthHandler(serviceName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := HealthResponse{
			Status:    "ok",
			Timestamp: time.Now().UTC(),
			Service:   serviceName,
		}

		json.NewEncoder(w).Encode(response)
	}
}

// readyHandler для readiness probes
func readyHandler(serviceName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Здесь можно добавить проверку подключения к БД
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		response := HealthResponse{
			Status:    "ready",
			Timestamp: time.Now().UTC(),
			Service:   serviceName,
		}

		json.NewEncoder(w).Encode(response)
	}
}

func main() {
	logrusLogger := logger.Init("graphql")

	cfg, err := config.Load()
	if err != nil {
		logrusLogger.WithError(err).Fatal("failed to load config")
	}

	logrusLogger.WithFields(map[string]interface{}{
		"port":    cfg.GraphQLPort,
		"db_host": cfg.DB.Host,
		"db_name": cfg.DB.DBName,
	}).Info("starting GraphQL service")

	var repo repository.TaskRepository
	if cfg.DB.Driver == "postgres" {
		postgresRepo, err := repository.NewPostgresTaskRepository(cfg.DB.DSN())
		if err != nil {
			logrusLogger.WithError(err).Fatal("failed to connect to database")
		}
		defer postgresRepo.Close()
		repo = postgresRepo
	} else {
		logrusLogger.Fatal("unsupported database driver: " + cfg.DB.Driver)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := repo.List(ctx); err != nil {
		logrusLogger.WithError(err).Warn("database not responding, but continuing")
	} else {
		logrusLogger.Info("database connected successfully")
	}

	resolver := graph.NewResolver(repo, logrusLogger)
	srv := handler.NewDefaultServer(graph.NewExecutableSchema(graph.Config{Resolvers: resolver}))

	// Создаём мультиплексор для маршрутов
	mux := http.NewServeMux()

	// Health checks для Kubernetes (хардкод, без зависимостей от shared)
	mux.HandleFunc("GET /health", healthHandler("graphql"))
	mux.HandleFunc("GET /ready", readyHandler("graphql"))

	// GraphQL endpoints
	mux.Handle("/", playground.Handler("GraphQL Playground", "/query"))
	mux.Handle("/query", srv)

	addr := fmt.Sprintf(":%s", cfg.GraphQLPort)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		logrusLogger.WithField("port", cfg.GraphQLPort).Info("GraphQL server started")
		logrusLogger.Info("Playground available at http://localhost:" + cfg.GraphQLPort + "/")
		logrusLogger.Info("Health endpoint available at http://localhost:" + cfg.GraphQLPort + "/health")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrusLogger.WithError(err).Fatal("server failed")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logrusLogger.Info("Shutting down GraphQL server...")

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logrusLogger.WithError(err).Fatal("server forced to shutdown")
	}

	logrusLogger.Info("server exited")
}
