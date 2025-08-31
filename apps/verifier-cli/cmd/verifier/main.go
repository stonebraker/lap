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
	case "verify":
		verifyCmd(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	exe := filepath.Base(os.Args[0])
	fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n", exe)
	fmt.Fprintf(os.Stderr, "\nCommands:\n")
	fmt.Fprintf(os.Stderr, "  verify      Verify a LAP v0.2 fragment located at the specified URL\n")
	fmt.Fprintf(os.Stderr, "\nVerification follows the v0.2 three-step process:\n")
	fmt.Fprintf(os.Stderr, "  1. Resource Presence - Check attestation accessibility and same-origin validation\n")
	fmt.Fprintf(os.Stderr, "  2. Resource Integrity - Verify content hash matches attestation\n")
	fmt.Fprintf(os.Stderr, "  3. Publisher Association - Validate namespace control and signature\n")
}

func verifyCmd(args []string) {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	urlFlag := fs.String("url", "", "absolute resource URL to verify")
	timeout := fs.Duration("timeout", 10*time.Second, "HTTP timeout")
	verbose := fs.Bool("v", false, "verbose output")
	jsonOutput := fs.Bool("json", false, "output structured JSON result matching v0.2 specification")
	_ = fs.Parse(args)
	
	if *urlFlag == "" {
		fmt.Fprintln(os.Stderr, "verify requires -url")
		fs.Usage()
		os.Exit(2)
	}

	opts := VerificationOptions{
		Timeout: *timeout,
		Verbose: *verbose,
	}

	result, err := VerifyResource(*urlFlag, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "verification error: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		// Output structured JSON matching v0.2 normative specification
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "json marshal error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(string(output))
	} else {
		// Human-readable output following v0.2 simplified approach
		if result.Verified {
			fmt.Printf("✅ Verification successful\n")
			fmt.Printf("  Resource Presence: %s\n", result.ResourcePresence)
			fmt.Printf("  Resource Integrity: %s\n", result.ResourceIntegrity)
			fmt.Printf("  Publisher Association: %s\n", result.PublisherAssociation)
		} else {
			fmt.Printf("❌ Verification failed\n")
			if result.Failure != nil {
				fmt.Printf("  Failed at: %s\n", result.Failure.Check)
				fmt.Printf("  Reason: %s\n", result.Failure.Reason)
				fmt.Printf("  Message: %s\n", result.Failure.Message)
			}
		}
		
		if *verbose && result.Context != nil {
			fmt.Printf("\nContext:\n")
			fmt.Printf("  Resource Attestation URL: %s\n", result.Context.ResourceAttestationURL)
			fmt.Printf("  Namespace Attestation URL: %s\n", result.Context.NamespaceAttestationURL)
			fmt.Printf("  Verified At: %d\n", result.Context.VerifiedAt)
		}
	}

	// Exit with appropriate code - simplified v0.2 approach
	if result.Verified {
		os.Exit(0)
	} else {
		os.Exit(1)
	}
}
