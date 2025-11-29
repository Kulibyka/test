package main

import (
	"backend/messages-service/internal/config"
	"backend/messages-service/internal/kafka"
	"backend/messages-service/internal/logger"
	"backend/messages-service/internal/messages"
	"backend/messages-service/internal/storage"
	"backend/messages-service/internal/storage/postgresql"
	messageshttp "backend/messages-service/internal/transport/http/messages"
	"context"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"
)

func main() {
	cfg := config.MustLoad()

	log := logger.New(cfg.Env)
	log.Info("starting app", slog.String("env", cfg.Env))

	if err := kafka.EnsureTopics(
		context.Background(),
		cfg.Kafka.Brokers,
		log,
		cfg.Kafka.InputTopic,
		cfg.Kafka.OutputTopic,
		cfg.Kafka.DeadLetterTopic,
	); err != nil {
		log.Error("failed to ensure kafka topics", slog.Any("error", err))
		panic(err)
	}

	dbStorage, err := postgresql.New(cfg.PostgreSQL)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := dbStorage.Close(); err != nil {
			log.Warn("failed to close postgresql connection", slog.Any("error", err))
		}
	}()

	repo := storage.NewMessagesRepo(dbStorage.DB)

	producer, err := kafka.NewProducer(cfg.Kafka, log)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := producer.Close(); err != nil {
			log.Warn("failed to close kafka producer", slog.Any("error", err))
		}
	}()

	svc := messages.NewService(
		repo,
		producer,
		log,
		cfg.Retries.MaxLLMAttempts,
		cfg.Kafka.InputTopic,
		cfg.Kafka.OutputTopic,
		cfg.Kafka.DeadLetterTopic,
		cfg.Org.FilePath,
	)

	handler := messageshttp.New(svc, log)

	mux := http.NewServeMux()
	handler.Register(mux)

	server := &http.Server{
		Addr:         cfg.HTTPServer.Address,
		Handler:      mux,
		ReadTimeout:  cfg.HTTPServer.Timeout,
		WriteTimeout: cfg.HTTPServer.Timeout,
		IdleTimeout:  cfg.HTTPServer.IdleTimeout,
	}

	log.Info("listening http", slog.String("address", cfg.HTTPServer.Address))

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http server error", slog.Any("error", err))
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	<-ctx.Done()
	log.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTPServer.Timeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", slog.Any("error", err))
	} else {
		log.Info("http server stopped")
	}
}
