// Command idc-compliance-gateway starts the installation-local compliance gateway.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"lina-platform/idc-compliance-gateway/internal/app"
	"lina-platform/idc-compliance-gateway/internal/buildinfo"
)

func main() {
	os.Exit(run())
}

func run() int {
	config, err := app.LoadConfig(os.Getenv)
	if err != nil {
		log.Printf("local compliance gateway configuration rejected: %v", err)
		return 1
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Run(ctx, config, buildinfo.Current()); err != nil {
		log.Printf("local compliance gateway stopped: %v", err)
		return 1
	}
	return 0
}
