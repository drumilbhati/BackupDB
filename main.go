package main

import (
	"os"

	"github.com/drumilbhati/BackupDB/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
