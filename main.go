package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/coalaura/lugo/lsp"
)

var Version = "dev"

func main() {
	ciFlag := flag.String("ci", "", "Path to CI configuration JSON file")
	flag.Parse()

	server := lsp.NewServer(Version)

	if *ciFlag != "" {
		os.Exit(server.RunCI(*ciFlag))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()

		os.Stdin.Close()
	}()

	err := server.Start()
	if err != nil {
		panic(err)
	}
}
