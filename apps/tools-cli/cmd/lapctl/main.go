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
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stonebraker/lap/apps/demo-utils/artifacts"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/crypto"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "keygen":
		keygenCmd(os.Args[2:])
	case "ra-create":
		raCreateCmd(os.Args[2:])
	case "fragment-create":
		fragmentCreateCmd(os.Args[2:])

	case "na-create":
		naCreateCmd(os.Args[2:])
	case "reset-artifacts":
		resetArtifactsCmd(os.Args[2:])
	case "verify-remote":
		verifyRemoteCmd(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	exe := filepath.Base(os.Args[0])
	fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n", exe)
	fmt.Fprintf(os.Stderr, "\nCommands:\n")
	fmt.Fprintf(os.Stderr, "  keygen      Generate a secp256k1 keypair and print or append to .env\n")
	fmt.Fprintf(os.Stderr, "  ra-create   Create a v0.2 resource attestation for an HTML file\n")
	fmt.Fprintf(os.Stderr, "  fragment-create   Create a v0.2 HTML fragment (index.htmx) from an content.htmx\n")

	fmt.Fprintf(os.Stderr, "  na-create     Create a v0.2 namespace attestation for a namespace URL\n")
	fmt.Fprintf(os.Stderr, "  reset-artifacts Reset all LAP artifacts for alice by creating a new NA and updating all posts\n")
	fmt.Fprintf(os.Stderr, "  verify-remote Fetch a fragment from a URL and verify it using the verifier service\n")
}

func keygenCmd(args []string) {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	name := fs.String("name", "alice", "label for the keypair (e.g. alice)")
	out := fs.String("out", "", "optional path to write env lines (e.g. .env)")
	_ = fs.Parse(args)

	priv, pubHex, err := crypto.GenerateKeyPair()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	_ = priv // not used other than to demonstrate generation
	privHex := hex.EncodeToString(priv.Serialize())

	prefix := *name
	if prefix == "" {
		prefix = "publisher"
	}
	lines := fmt.Sprintf("%s=%s\n%s=%s\n", envKey(prefix, "PRIVKEY"), privHex, envKey(prefix, "PUBKEY_XONLY"), pubHex)

	if *out == "" {
		fmt.Print(lines)
		return
	}
	f, err := os.OpenFile(*out, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open %s: %v\n", *out, err)
		os.Exit(1)
	}
	defer f.Close()
	if _, err := f.WriteString(lines); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", *out, err)
		os.Exit(1)
	}
}





func raCreateCmd(args []string) {
	fs := flag.NewFlagSet("ra-create", flag.ExitOnError)
	inPath := fs.String("in", "", "path to input HTML file")
	resURL := fs.String("url", "", "absolute resource URL or path (e.g. https://example.com/path or /people/alice/posts/1)")
	base := fs.String("base", "", "optional base (scheme://host[:port]) to resolve -url against, e.g. http://localhost:8080")
	publisherClaim := fs.String("publisher-claim", "", "publisher's secp256k1 X-only public key (64 hex chars) for triangulation")
	namespaceAttestationURL := fs.String("namespace-attestation-url", "", "URL pointing to the Namespace Attestation (required)")
	out := fs.String("out", "", "output file path (default: <dir>/_la_resource.json)")
	_ = fs.Parse(args)

	if *inPath == "" || *resURL == "" || *publisherClaim == "" || *namespaceAttestationURL == "" {
		fmt.Fprintf(os.Stderr, "ra-create requires -in, -url, -publisher-claim, and -namespace-attestation-url\n")
		fs.Usage()
		os.Exit(2)
	}

	err := artifacts.CreateResourceAttestation(*inPath, *resURL, *base, *publisherClaim, *namespaceAttestationURL, *out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Fprintf(os.Stderr, "wrote %s\n", *out)
}

func fragmentCreateCmd(args []string) {
	fs := flag.NewFlagSet("fragment-create", flag.ExitOnError)
	inPath := fs.String("in", "", "path to input content.htmx file")
	resURL := fs.String("url", "", "absolute resource URL or path (e.g. https://example.com/path or /people/alice/messages/1)")
	base := fs.String("base", "", "optional base (scheme://host[:port]) to resolve -url against, e.g. http://localhost:8080")
	publisherClaim := fs.String("publisher-claim", "", "publisher's secp256k1 X-only public key (64 hex chars) for triangulation")
	resourceAttestationURL := fs.String("resource-attestation-url", "", "URL pointing to the Resource Attestation (required)")
	namespaceAttestationURL := fs.String("namespace-attestation-url", "", "URL pointing to the Namespace Attestation (required)")
	out := fs.String("out", "", "output fragment HTML path (default: <dir>/index.htmx)")
	updateHost := fs.String("update", "", "optional path to host HTML file whose matching <article data-la-fragment-url> should be replaced with the new fragment")
	dryRun := fs.Bool("dry-run", false, "if set, do not write changes to -update host file; just report action")
	_ = fs.Parse(args)

	if *inPath == "" || *resURL == "" || *publisherClaim == "" || *resourceAttestationURL == "" || *namespaceAttestationURL == "" {
		fmt.Fprintf(os.Stderr, "fragment-create requires -in, -url, -publisher-claim, -resource-attestation-url, and -namespace-attestation-url\n")
		fs.Usage()
		os.Exit(2)
	}

	err := artifacts.CreateFragment(*inPath, *resURL, *base, *publisherClaim, *resourceAttestationURL, *namespaceAttestationURL, *out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "wrote %s\n", *out)

	if *updateHost != "" {
		// Read the created fragment to get the fragment URL and HTML
		fragmentBytes, err := os.ReadFile(*out)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read fragment %s: %v\n", *out, err)
			os.Exit(1)
		}
		
		// Extract fragment URL from the fragment HTML
		// This is a simplified approach - in practice you might want to parse the HTML
		// For now, we'll use the resURL as the fragment URL
		fragmentURL := *resURL
		if *base != "" {
			// Build the full URL like in CreateFragment
			baseURL, err := url.Parse(*base)
			if err == nil && baseURL.Scheme != "" && baseURL.Host != "" {
				rawU, err := url.Parse(*resURL)
				if err == nil {
					baseURL.Path = rawU.Path
					baseURL.RawQuery = rawU.RawQuery
					fragmentURL = baseURL.String()
				}
			}
		}
		
		if *dryRun {
			fmt.Fprintf(os.Stderr, "update: would write %s (dry-run)\n", *updateHost)
			return
		}
		
		err = artifacts.UpdateHostFile(*updateHost, fragmentURL, string(fragmentBytes))
		if err != nil {
			fmt.Fprintf(os.Stderr, "update error: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "updated %s\n", *updateHost)
	}
}



func naCreateCmd(args []string) {
	fs := flag.NewFlagSet("na-create", flag.ExitOnError)
	namespace := fs.String("namespace", "", "namespace URL (e.g. https://example.com/people/alice/)")
	expStr := fs.String("exp", "", "expiration timestamp in seconds since epoch (default: 1 year from now)")
	privHexFlag := fs.String("privkey", "", "(optional) hex-encoded publisher private key; if provided, will be used and stored")
	out := fs.String("out", "", "output directory path (default: current directory)")

	keysDir := fs.String("keys-dir", "keys", "directory to store per-namespace keys (outside static)")
	rotate := fs.Bool("rotate", false, "force generating a new keypair even if one exists for this namespace")
	_ = fs.Parse(args)

	if *namespace == "" {
		fmt.Fprintf(os.Stderr, "na-create requires -namespace\n")
		fs.Usage()
		os.Exit(2)
	}

	outputPath, err := artifacts.CreateNamespaceAttestation(*namespace, *expStr, *privHexFlag, *out, *keysDir, *rotate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Created namespace attestation at %s\n", outputPath)
}



func envKey(prefix, key string) string {
	return fmt.Sprintf("%s_%s", toUpper(prefix), key)
}

func toUpper(s string) string {
	b := []byte(s)
	for i := range b {
		c := b[i]
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}



// resetArtifactsCmd resets all LAP artifacts for alice by creating a new NA and updating all posts
func resetArtifactsCmd(args []string) {
	fs := flag.NewFlagSet("reset-artifacts", flag.ExitOnError)
	base := fs.String("base", "http://localhost:8080", "base URL (scheme://host[:port]) for LAP URLs")
	root := fs.String("root", "apps/server/static/publisherapi/people/alice", "root directory for Alice content")
	keysDir := fs.String("keys-dir", "keys", "directory containing publisher keys")
	_ = fs.Parse(args)

	err := artifacts.ResetArtifacts(*base, *root, *keysDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// verifyRemoteCmd fetches a fragment from a URL and verifies it using the verifier service
func verifyRemoteCmd(args []string) {
	fs := flag.NewFlagSet("verify-remote", flag.ExitOnError)
	fragmentURL := fs.String("url", "", "URL of the fragment to fetch and verify")
	verifierURL := fs.String("verifier", "http://localhost:8082", "base URL of the verifier service")
	timeout := fs.Duration("timeout", 30*time.Second, "HTTP timeout for requests")
	_ = fs.Parse(args)

	if *fragmentURL == "" {
		fmt.Fprintf(os.Stderr, "verify-remote requires -url\n")
		fs.Usage()
		os.Exit(2)
	}

	// Create HTTP client
	client := &http.Client{
		Timeout: *timeout,
	}

	// Fetch the fragment from the specified URL
	fmt.Fprintf(os.Stderr, "Fetching fragment from %s...\n", *fragmentURL)
	resp, err := client.Get(*fragmentURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching fragment: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Fprintf(os.Stderr, "error fetching fragment: HTTP %d %s\n", resp.StatusCode, resp.Status)
		os.Exit(1)
	}

	// Read the fragment content
	fragmentContent, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading fragment content: %v\n", err)
		os.Exit(1)
	}

	// Post the fragment to the verifier service
	verifyEndpoint := strings.TrimSuffix(*verifierURL, "/") + "/verify"
	fmt.Fprintf(os.Stderr, "Posting fragment to verifier service at %s...\n", verifyEndpoint)
	
	verifyResp, err := client.Post(verifyEndpoint, "text/html", bytes.NewReader(fragmentContent))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error posting to verifier service: %v\n", err)
		os.Exit(1)
	}
	defer verifyResp.Body.Close()

	// Read the verification result
	resultContent, err := io.ReadAll(verifyResp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading verification result: %v\n", err)
		os.Exit(1)
	}

	// Pretty print the JSON result
	var result map[string]interface{}
	if err := json.Unmarshal(resultContent, &result); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing verification result: %v\n", err)
		fmt.Fprintf(os.Stderr, "Raw response: %s\n", string(resultContent))
		os.Exit(1)
	}

	// Format and display the result
	prettyResult, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error formatting result: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(prettyResult))

	// Exit with appropriate code based on verification result
	if verified, ok := result["verified"].(bool); ok && verified {
		os.Exit(0)
	} else {
		os.Exit(1)
	}
}
