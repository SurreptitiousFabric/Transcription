package main

import (
	"fmt"
	"os"

	"testme/ui"
)

func main() {
	if err := ui.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
