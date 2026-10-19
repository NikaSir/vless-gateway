package main

import (
	"context"
	"fmt"
	"github.com/NikaSir/vless-gateway/internal/app"
	"os"
)

func main() {
	if err := app.RunCLI(context.Background(), os.Args[1:], os.Stdout, os.Geteuid()); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
