package main

import (
	"flag"
	"fmt"
	"os"
)

func run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("expected command: init, open, info")
	}

	switch args[0] {
	case "init":
		return fmt.Errorf("not yet implemented")
	case "open":
		return fmt.Errorf("not yet implemented")
	case "info":
		return fmt.Errorf("not yet implemented")
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func main() {
	flag.Parse()
	if err := run(flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
