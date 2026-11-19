package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/UcGeorge-Canonical/easy-lima/internal/ui/cli"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(cli.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
