package main

import (
	"os"

	"github.com/DawnKosmos/gotzer/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
