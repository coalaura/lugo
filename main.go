package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/coalaura/lugo/lsp"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server := lsp.NewServer()

	go func() {
		<-ctx.Done()

		os.Stdin.Close()
	}()

	err := server.Start()
	if err != nil {
		panic(err)
	}
}
