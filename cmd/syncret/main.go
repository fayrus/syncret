package main

import (
	"context"
	"log/slog"
	"os"
	_ "time/tzdata" // embedded for Lambda environments without a timezone database

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"

	internalaws "github.com/fayrus/syncret/internal/aws"
	"github.com/fayrus/syncret/internal/config"
	"github.com/fayrus/syncret/internal/handler"
)

var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("config: invalid", "error", err)
		os.Exit(1)
	}

	setupLogger(cfg.LogLevel, cfg.LogFormat)
	slog.Info("syncret starting", "version", version)

	awsCfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(cfg.AWSRegion))
	if err != nil {
		slog.Error("aws: load config", "error", err)
		os.Exit(1)
	}

	smClient := internalaws.NewSecretsManager(secretsmanager.NewFromConfig(awsCfg))
	ecsClient := internalaws.NewECS(ecs.NewFromConfig(awsCfg))

	deps := handler.Dependencies{
		Secrets: smClient,
		ECS:     ecsClient,
	}

	handler.StartLambda(cfg, deps)
}

func setupLogger(level, format string) {
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		slog.Warn("invalid log level, defaulting to info", "value", level)
	}

	opts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	if format == "text" {
		h = slog.NewTextHandler(os.Stdout, opts)
	} else {
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(h))
}
