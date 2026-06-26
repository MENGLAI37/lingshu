package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lingshu/lingshu/pkg/tui/models"
)

var Version = "v0.1.0"

func main() {
	noTUI := flag.Bool("no-tui", false, "Disable TUI mode, use plain text output")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("lingshu version %s\n", Version)
		os.Exit(0)
	}

	if *noTUI {
		runNoTUI()
		return
	}

	if err := runTUI(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func runTUI() error {
	model := models.NewTUIModel()
	model.SetCluster("kind-lingshu-dev")
	model.SetNamespace("default")
	model.SetEnvironment("development")

	p := tea.NewProgram(model, tea.WithAltScreen())
	model.SetProgram(p)

	_, err := p.Run()
	return err
}

func runNoTUI() {
	fmt.Println("lingshu - AI-native SRE Agent")
	fmt.Printf("Version: %s\n", Version)
	fmt.Println("Mode: No-TUI (plain text)")
	fmt.Println("\nThis mode is under development.")
	fmt.Println("Please run without --no-tui flag to use the TUI interface.")
}
