package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	fmt.Println("Building LAP project...")
	
	// Create bin directory
	if err := os.MkdirAll("bin", 0755); err != nil {
		fmt.Printf("Error creating bin directory: %v\n", err)
		os.Exit(1)
	}
	
	// Define all the main packages to build
	buildTargets := []struct {
		name string
		path string
	}{
		{"lapctl", "./apps/tools-cli/cmd/lapctl"},
		{"publisherapi", "./apps/server/cmd/publisherapi"},
		{"client-server", "./apps/client-server/cmd/client-server"},
		{"verifier", "./apps/verifier-cli/cmd/verifier"},
		{"verifier-service", "./apps/verifier-service/cmd/verifier-service"},
	}
	
	// Build each target
	for _, target := range buildTargets {
		fmt.Printf("Building %s...\n", target.name)
		
		outputPath := filepath.Join("bin", target.name)
		cmd := exec.Command("go", "build", "-o", outputPath, target.path)
		
		// Capture both stdout and stderr
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		
		if err := cmd.Run(); err != nil {
			fmt.Printf("Error building %s: %v\n", target.name, err)
			os.Exit(1)
		}
		
		fmt.Printf("✓ Built %s\n", target.name)
	}
	
	fmt.Println("\nAll binaries built successfully!")
	fmt.Println("Binaries are located in the bin/ directory:")
	
	// List the built binaries
	if entries, err := os.ReadDir("bin"); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				fmt.Printf("  - bin/%s\n", entry.Name())
			}
		}
	}
}
