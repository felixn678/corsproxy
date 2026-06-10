package main

import (
	"os"

	"github.com/felix-nguyen/corsproxy/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
