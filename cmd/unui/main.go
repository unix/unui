package main

import (
	"os"

	"github.com/unix/unui-cli/internal/command"
)

func main() {
	os.Exit(command.Execute(os.Args[1:], os.Stdout, os.Stderr))
}
