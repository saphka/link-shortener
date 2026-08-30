package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/saphka/link-shortener/internal/app"
	"github.com/saphka/link-shortener/internal/config"
	"github.com/stoewer/go-strcase"
)

func main() {
	err := run()
	if err != nil {
		log.Fatalf("error running app: %v", err)
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	name := "link"

	cfg, err := config.Load(ctx, strcase.UpperSnakeCase(name)+"_")
	if err != nil {
		return fmt.Errorf("cannot load config: %w", err)
	}

	app, err := app.NewApp(ctx, "link", cfg)
	if err != nil {
		return fmt.Errorf("cannot create app: %w", err)
	}
	app.Run(ctx)
	return nil
}
