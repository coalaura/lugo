package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/coalaura/lugo/lsp"
)

var Version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := lsp.NewServer(Version)

	go func() {
		<-ctx.Done()

		os.Stdin.Close()
	}()

	err := server.Start()
	if err != nil {
		panic(err)
	}
}
