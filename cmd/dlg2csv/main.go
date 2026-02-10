package main

import (
	"fmt"
	"os"

	"github.com/maciejjwojcik/dlg2csv/internal/csv"
	"github.com/maciejjwojcik/dlg2csv/internal/d"
	"github.com/maciejjwojcik/dlg2csv/internal/tra"
)

func main() {
	dir := "."
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	fmt.Println("Parsing .tra files...")
	traByFile, err := tra.ParseDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "TRA parse error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Parsing .d files...")
	dByFile, err := d.ParseDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "D parse error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Exporting CSV...")
	if _, err := csv.Export(dByFile, traByFile); err != nil {
		fmt.Fprintf(os.Stderr, "Export error: %v\n", err)
		os.Exit(1)
	}

	cwd, _ := os.Getwd()
	fmt.Println("CWD:", cwd)
	fmt.Println("TRA files:", len(traByFile))
	fmt.Println("D files:", len(dByFile))

	for k := range traByFile {
		fmt.Println("TRA key:", k)
		break
	}
	for k := range dByFile {
		fmt.Println("D key:", k)
		break
	}

	fmt.Println("Done.")
}
