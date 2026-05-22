package main

import (
	"os"

	"github.com/e12media/satip-lab/internal/simctl"
)

func main() {
	os.Exit(simctl.Run(os.Args[1:], os.Stdout, os.Stderr))
}
