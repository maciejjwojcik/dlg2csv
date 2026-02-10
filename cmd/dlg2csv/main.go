package main

import (
	"fmt"
	"os"

	"github.com/maciejjwojcik/dlg2csv/internal/csv"
	"github.com/maciejjwojcik/dlg2csv/internal/d"
	"github.com/maciejjwojcik/dlg2csv/internal/tra"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage:\n  %s                # read .tra and .d from current directory\n  %s <traDir> <dDir> # read .tra from traDir and .d from dDir\n", os.Args[0], os.Args[0])
}

func main() {
	args := os.Args[1:]

	traDir := "."
	dDir := "."

	switch len(args) {
	case 0:
		// defaults already set
	case 2:
		traDir = args[0]
		dDir = args[1]
	default:
		usage()
		fmt.Fprintf(os.Stderr, "\nError: expected 0 or 2 arguments, got %d\n", len(args))
		os.Exit(2)
	}

	fmt.Println("Parsing .tra files from:", traDir)
	traByFile, err := tra.ParseDir(traDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "TRA parse error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Parsing .d files from:", dDir)
	dByFile, err := d.ParseDir(dDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "D parse error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Exporting CSV...")
	if _, err := csv.Export(dByFile, traByFile); err != nil {
		fmt.Fprintf(os.Stderr, "Export error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Done.")
}
