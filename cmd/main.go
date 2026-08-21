package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/saphka/link-shortener/internal/app"
	"github.com/saphka/link-shortener/internal/config"
	"github.com/stoewer/go-strcase"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	name := "link"

	cfg, err := config.Load(ctx, strcase.UpperSnakeCase(name)+"_")
	if err != nil {
		log.Fatalf("cannot load config: %v", err)
	}

	app, err := app.NewApp(ctx, "link", cfg)
	if err != nil {
		log.Fatalf("cannot create app: %s", err)
	}
	app.Run()
}
