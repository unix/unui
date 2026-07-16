package main

import (
	"os"

	"github.com/unix/unui/internal/command"
)

func main() {
	os.Exit(command.Execute(os.Args[1:], os.Stdout, os.Stderr))
}
