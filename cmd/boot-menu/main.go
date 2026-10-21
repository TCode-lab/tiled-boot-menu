// Command boot-menu starts the application launcher.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"bootmenu/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, os.Args[1:], os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "boot-menu:", err)
		os.Exit(1)
	}
}
