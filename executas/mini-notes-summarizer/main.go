package main

import (
	"context"
	"fmt"
	"os"

	"mini-notes-summarizer/internal/executa"
)

func main() {
	server := executa.NewServer(os.Stdin, os.Stdout, os.Stderr)
	if err := server.Run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "server stopped: %v\n", err)
		os.Exit(1)
	}
}
