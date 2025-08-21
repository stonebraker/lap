// Copyright 2025 Jason Stonebraker
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "help", "-h", "--help":
		usage()
	case "ra-verify":
		raVerifyCmd(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	exe := filepath.Base(os.Args[0])
	fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n", exe)
	fmt.Fprintf(os.Stderr, "\nCommands:\n")
	fmt.Fprintf(os.Stderr, "  ra-verify   Verify a resource attestation for a fetched HTML file\n")
}

func raVerifyCmd(args []string) {
	fs := flag.NewFlagSet("ra-verify", flag.ExitOnError)
	urlFlag := fs.String("url", "", "absolute resource URL to verify")
	timeout := fs.Duration("timeout", 10*time.Second, "HTTP timeout")
	verbose := fs.Bool("v", false, "verbose output")
	policy := fs.String("policy", "strict", "verification policy: strict, graceful, offline-fallback, auto-refresh")
	skewSeconds := fs.Int("skew", 120, "grace period in seconds for graceful policy")
	jsonOutput := fs.Bool("json", false, "output structured JSON result")
	_ = fs.Parse(args)
	
	if *urlFlag == "" {
		fmt.Fprintln(os.Stderr, "ra-verify requires -url")
		fs.Usage()
		os.Exit(2)
	}

	opts := VerificationOptions{
		Policy:      *policy,
		SkewSeconds: *skewSeconds,
		Timeout:     *timeout,
		Verbose:     *verbose,
	}

	result, err := VerifyResource(*urlFlag, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verification error: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		// Output structured JSON matching JavaScript verifier.core.js format
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "json marshal error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(output))
	} else {
		// Human-readable output
		if result.OK {
			fmt.Printf("✅ %s: %s\n", result.Code, result.Message)
			if result.Status == "warn" {
				fmt.Printf("⚠️  Warning: %s\n", result.Message)
			}
		} else {
			fmt.Printf("❌ %s: %s\n", result.Code, result.Message)
		}
		
		if *verbose {
			fmt.Printf("\nTelemetry:\n")
			fmt.Printf("  URL: %s\n", result.Telemetry.URL)
			fmt.Printf("  Policy: %s\n", result.Telemetry.Policy) 
			fmt.Printf("  Kid: %s\n", result.Telemetry.Kid)
			if result.Telemetry.IAT != nil {
				fmt.Printf("  IAT: %d\n", *result.Telemetry.IAT)
			}
			if result.Telemetry.EXP != nil {
				fmt.Printf("  EXP: %d\n", *result.Telemetry.EXP)
			}
			fmt.Printf("  Now: %d\n", result.Telemetry.Now)
		}
	}

	// Exit with appropriate code
	if result.OK {
		os.Exit(0)
	} else {
		os.Exit(1) 
	}
}

// Helpers are now in verification.go
