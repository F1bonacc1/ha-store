package main

import (
	"os"

	"github.com/f1bonacc1/ha-store/stctl/cmd"
)

func main() {
	if err := cmd.NewRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
