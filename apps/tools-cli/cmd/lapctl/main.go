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
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/canonical"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/crypto"
	"github.com/stonebraker/lap/sdks/go/pkg/lap/wire"
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
	case "update-posts":
		updatePostsCmd(os.Args[2:])
	case "na-create":
		naCreateCmd(os.Args[2:])
	case "reset-artifacts":
		resetArtifactsCmd(os.Args[2:])
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
	fmt.Fprintf(os.Stderr, "  update-posts      Generate v0.2 fragments for posts 1..3\n")
	fmt.Fprintf(os.Stderr, "  na-create     Create a v0.2 namespace attestation for a namespace URL\n")
	fmt.Fprintf(os.Stderr, "  reset-artifacts Reset all LAP artifacts for alice by creating a new NA and updating all posts\n")
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

type storedKey struct {
	PrivKeyHex    string `json:"privkey_hex"`
	PubKeyXOnly   string `json:"pubkey_xonly_hex"`
	CreatedAtUnix int64  `json:"created_at"`
}



func raCreateCmd(args []string) {
	fs := flag.NewFlagSet("ra-create", flag.ExitOnError)
	inPath := fs.String("in", "", "path to input HTML file")
	resURL := fs.String("url", "", "absolute resource URL or path (e.g. https://example.com/path or /people/alice/posts/1)")
	base := fs.String("base", "", "optional base (scheme://host[:port]) to resolve -url against, e.g. http://localhost:8081")
	publisherClaim := fs.String("publisher-claim", "", "publisher's secp256k1 X-only public key (64 hex chars) for triangulation")
	namespaceAttestationURL := fs.String("namespace-attestation-url", "", "URL pointing to the Namespace Attestation (required)")
	out := fs.String("out", "", "output file path (default: <dir>/_la_resource.json)")
	_ = fs.Parse(args)

	if *inPath == "" || *resURL == "" || *publisherClaim == "" || *namespaceAttestationURL == "" {
		fmt.Fprintf(os.Stderr, "ra-create requires -in, -url, -publisher-claim, and -namespace-attestation-url\n")
		fs.Usage()
		os.Exit(2)
	}

	body, err := os.ReadFile(*inPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", *inPath, err)
		os.Exit(1)
	}

	// Build payload URL with optional base override
	var u url.URL
	if *base != "" {
		baseURL, err := url.Parse(*base)
		if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
			fmt.Fprintf(os.Stderr, "invalid -base: %s\n", *base)
			os.Exit(2)
		}
		u = *baseURL
		// take path+query from resURL (absolute or relative)
		rawU, err := url.Parse(*resURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid -url: %s\n", *resURL)
			os.Exit(2)
		}
		if rawU.Path != "" {
			u.Path = rawU.Path
		}
		u.RawQuery = rawU.RawQuery
	} else {
		rawU, err := url.Parse(*resURL)
		if err != nil || rawU.Scheme == "" || rawU.Host == "" {
			fmt.Fprintf(os.Stderr, "invalid -url (expect absolute when -base not set): %s\n", *resURL)
			os.Exit(2)
		}
		u = *rawU
	}

	// Canonicalize scheme/host (lower) and strip default ports
	u.Scheme = strings.ToLower(u.Scheme)
	hu := strings.ToLower(u.Host)
	if (u.Scheme == "http" && strings.HasSuffix(hu, ":80")) || (u.Scheme == "https" && strings.HasSuffix(hu, ":443")) {
		hu = strings.Split(hu, ":")[0]
	}
	u.Host = hu
	payloadURL := u.String()

	// Create v0.2 Resource Attestation
	att := wire.ResourceAttestation{
		FragmentURL:             payloadURL,
		Hash:                    crypto.ComputeContentHashField(body),
		PublisherClaim:          *publisherClaim,
		NamespaceAttestationURL: *namespaceAttestationURL,
	}

	// Output
	outPath := *out
	if outPath == "" {
		dir := filepath.Dir(*inPath)
		outPath = filepath.Join(dir, "_la_resource.json")
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	f, err := os.Create(outPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create %s: %v\n", outPath, err)
		os.Exit(1)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(att); err != nil {
		fmt.Fprintf(os.Stderr, "write json: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Fprintf(os.Stderr, "wrote %s\n", outPath)
}

func fragmentCreateCmd(args []string) {
	fs := flag.NewFlagSet("fragment-create", flag.ExitOnError)
	inPath := fs.String("in", "", "path to input content.htmx file")
	resURL := fs.String("url", "", "absolute resource URL or path (e.g. https://example.com/path or /people/alice/messages/1)")
	base := fs.String("base", "", "optional base (scheme://host[:port]) to resolve -url against, e.g. http://localhost:8081")
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

	body, err := os.ReadFile(*inPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", *inPath, err)
		os.Exit(2)
	}

	// Build payload URL with optional base override
	var u url.URL
	if *base != "" {
		baseURL, err := url.Parse(*base)
		if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
			fmt.Fprintf(os.Stderr, "invalid -base: %s\n", *base)
			os.Exit(2)
		}
		u = *baseURL
		// take path+query from resURL (absolute or relative)
		rawU, err := url.Parse(*resURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid -url: %s\n", *resURL)
			os.Exit(2)
		}
		if rawU.Path != "" {
			u.Path = rawU.Path
		}
		u.RawQuery = rawU.RawQuery
	} else {
		rawU, err := url.Parse(*resURL)
		if err != nil || rawU.Scheme == "" || rawU.Host == "" {
			fmt.Fprintf(os.Stderr, "invalid -url (expect absolute when -base not set): %s\n", *resURL)
			os.Exit(2)
		}
		u = *rawU
	}

	// Canonicalize scheme/host (lower) and strip default ports
	u.Scheme = strings.ToLower(u.Scheme)
	hu := strings.ToLower(u.Host)
	if (u.Scheme == "http" && strings.HasSuffix(hu, ":80")) || (u.Scheme == "https" && strings.HasSuffix(hu, ":443")) {
		hu = strings.Split(hu, ":")[0]
	}
	u.Host = hu
	payloadURL := u.String()

	// Build v0.2 fragment HTML structure
	// Base64 of the exact canonical body bytes
	b64 := base64.StdEncoding.EncodeToString(body)

	article := "" +
		"<article\n" +
		"  data-la-spec=\"v0.2\"\n" +
		fmt.Sprintf("  data-la-fragment-url=\"%s\"\n", payloadURL) +
		">\n\n" +
		"  <section class=\"la-preview\">\n" +
		string(body) + "\n" +
		"  </section>\n\n" +
		"  <link\n" +
		"    rel=\"canonical\"\n" +
		"    type=\"text/html\"\n" +
		fmt.Sprintf("    data-la-publisher-claim=\"%s\"\n", *publisherClaim) +
		fmt.Sprintf("    data-la-resource-attestation-url=\"%s\"\n", *resourceAttestationURL) +
		fmt.Sprintf("    data-la-namespace-attestation-url=\"%s\"\n", *namespaceAttestationURL) +
		fmt.Sprintf("    href=\"data:text/html;base64,%s\"\n", b64) +
		"    hidden\n" +
		"  />\n" +
		"</article>\n"

	fragOut := *out
	if fragOut == "" {
		fragOut = filepath.Join(filepath.Dir(*inPath), "index.htmx")
	}
	if err := os.MkdirAll(filepath.Dir(fragOut), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(fragOut, []byte(article), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", fragOut, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "wrote %s\n", fragOut)

	if *updateHost != "" {
		hostBytes, err := os.ReadFile(*updateHost)
		if err != nil {
			fmt.Fprintf(os.Stderr, "update read %s: %v\n", *updateHost, err)
			os.Exit(1)
		}
		replaced, ok := replaceArticleByDataLaFragmentURL(string(hostBytes), payloadURL, article)
		if !ok {
			fmt.Fprintf(os.Stderr, "update: did not find <article> with data-la-fragment-url=\"%s\" in %s\n", payloadURL, *updateHost)
			os.Exit(2)
		}
		if *dryRun {
			fmt.Fprintf(os.Stderr, "update: would write %s (dry-run)\n", *updateHost)
			return
		}
		if err := os.WriteFile(*updateHost+".bak", hostBytes, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "backup %s: %v\n", *updateHost+".bak", err)
			os.Exit(1)
		}
		if err := os.WriteFile(*updateHost, []byte(replaced), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "update write %s: %v\n", *updateHost, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "updated %s\n", *updateHost)
	}
}

// updatePostsCmd automatically generates LAP fragments and resource attestations for posts 1..3
// based on LAP conventions, then updates the host file with the new fragments.
// Usage:
//   lapctl update-posts -base https://example.com -dir apps/server/static/publisherapi/people/alice
func updatePostsCmd(args []string) {
    fs := flag.NewFlagSet("update-posts", flag.ExitOnError)
    base := fs.String("base", "http://localhost:8081", "base URL (scheme://host[:port]) for LAP URLs")
    root := fs.String("dir", "apps/server/static/publisherapi/people/alice", "root directory for Alice content")
    _ = fs.Parse(args)

    // Load the publisher key from the keys directory
    keysDir := "keys"
    aliceKeyPath := filepath.Join(keysDir, "alice_publisher_key.json")
    
    var publisherKey string
    if data, err := os.ReadFile(aliceKeyPath); err == nil {
        var stored storedKey
        if json.Unmarshal(data, &stored) == nil && stored.PubKeyXOnly != "" {
            publisherKey = stored.PubKeyXOnly
        }
    }
    
    if publisherKey == "" {
        fmt.Fprintf(os.Stderr, "error: could not load publisher key from %s\n", aliceKeyPath)
        fmt.Fprintf(os.Stderr, "please create this key first using: lapctl keygen -name alice -out %s\n", aliceKeyPath)
        os.Exit(1)
    }

    // Construct LAP URLs based on conventions
    namespaceAttestationURL := fmt.Sprintf("%s/people/alice/_la_namespace.json", *base)
    
    // Process each post
    for postNum := 1; postNum <= 3; postNum++ {
        postDir := filepath.Join(*root, "posts", strconv.Itoa(postNum))
        inPath := filepath.Join(postDir, "content.htmx")
        outPath := filepath.Join(postDir, "index.htmx")
        
        // Construct URLs for this post
        fragmentURL := fmt.Sprintf("%s/people/alice/posts/%d", *base, postNum)
        resourceAttestationURL := fmt.Sprintf("%s/people/alice/posts/%d/_la_resource.json", *base, postNum)
        
        // Generate resource attestation first
        fmt.Fprintf(os.Stderr, "generating resource attestation for post %d...\n", postNum)
        raArgs := []string{
            "ra-create",
            "-in", inPath,
            "-url", fragmentURL,
            "-publisher-claim", publisherKey,
            "-namespace-attestation-url", namespaceAttestationURL,
            "-out", filepath.Join(postDir, "_la_resource.json"),
        }
        
        cmd := exec.Command(os.Args[0], raArgs...)
        cmd.Stdout = os.Stdout
        cmd.Stderr = os.Stderr
        if err := cmd.Run(); err != nil {
            fmt.Fprintf(os.Stderr, "error generating RA for post %d: %v\n", postNum, err)
            os.Exit(1)
        }
        
        // Generate fragment
        fmt.Fprintf(os.Stderr, "generating fragment for post %d...\n", postNum)
        fragmentArgs := []string{
            "fragment-create",
            "-in", inPath,
            "-url", fragmentURL,
            "-publisher-claim", publisherKey,
            "-resource-attestation-url", resourceAttestationURL,
            "-namespace-attestation-url", namespaceAttestationURL,
            "-out", outPath,
        }
        
        cmd = exec.Command(os.Args[0], fragmentArgs...)
        cmd.Stdout = os.Stdout
        cmd.Stderr = os.Stderr
        if err := cmd.Run(); err != nil {
            fmt.Fprintf(os.Stderr, "error generating fragment for post %d: %v\n", postNum, err)
            os.Exit(1)
        }
    }
    
    // Now update the host file with all three fragments
    hostPath := filepath.Join(*root, "posts", "index.html")
    fmt.Fprintf(os.Stderr, "updating host file %s with new fragments...\n", hostPath)
    
    // Read the host file
    hostBytes, err := os.ReadFile(hostPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "error reading host file %s: %v\n", hostPath, err)
        os.Exit(1)
    }
    
    // Update each post fragment in the host file
    for postNum := 1; postNum <= 3; postNum++ {
        fragmentPath := filepath.Join(*root, "posts", strconv.Itoa(postNum), "index.htmx")
        fragmentBytes, err := os.ReadFile(fragmentPath)
        if err != nil {
            fmt.Fprintf(os.Stderr, "error reading fragment %s: %v\n", fragmentPath, err)
            os.Exit(1)
        }
        
        fragmentURL := fmt.Sprintf("%s/people/alice/posts/%d", *base, postNum)
        replaced, ok := replaceArticleByDataLaFragmentURL(string(hostBytes), fragmentURL, string(fragmentBytes))
        if !ok {
            fmt.Fprintf(os.Stderr, "warning: could not find article with data-la-fragment-url=\"%s\" in host file\n", fragmentURL)
            continue
        }
        hostBytes = []byte(replaced)
    }
    
    // Write the updated host file
    if err := os.WriteFile(hostPath+".bak", hostBytes, 0644); err != nil {
        fmt.Fprintf(os.Stderr, "error creating backup: %v\n", err)
        os.Exit(1)
    }
    
    if err := os.WriteFile(hostPath, hostBytes, 0644); err != nil {
        fmt.Fprintf(os.Stderr, "error writing updated host file: %v\n", err)
        os.Exit(1)
    }
    
    fmt.Fprintf(os.Stderr, "successfully updated host file %s\n", hostPath)
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

	// Parse or set expiration timestamp
	var exp int64
	var err error
	if *expStr != "" {
		exp, err = strconv.ParseInt(*expStr, 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid -exp: %v\n", err)
			os.Exit(2)
		}
	} else {
		// Default to 1 year from now
		exp = time.Now().AddDate(1, 0, 0).Unix()
	}

	// Get or generate private key
	var priv *btcec.PrivateKey
	var pubHex string

	if *privHexFlag != "" {
		priv, err = crypto.ParsePrivateKeyHex(*privHexFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid -privkey: %v\n", err)
			os.Exit(2)
		}
		pub := priv.PubKey()
		pubHex = hex.EncodeToString(schnorr.SerializePubKey(pub))
	} else {
		// Check if this is for Alice's namespace and use her specific key
		if strings.Contains(*namespace, "/people/alice/") {
			aliceKeyPath := filepath.Join(*keysDir, "alice_publisher_key.json")
			if data, err := os.ReadFile(aliceKeyPath); err == nil {
				var stored storedKey
				if json.Unmarshal(data, &stored) == nil {
					priv, err = crypto.ParsePrivateKeyHex(stored.PrivKeyHex)
					if err == nil {
						pubHex = stored.PubKeyXOnly
						fmt.Fprintf(os.Stderr, "Using Alice's publisher key: %s\n", pubHex)
					}
				}
			}
		}
		
		// If not Alice or Alice key not found, try to load existing key from keys directory
		if priv == nil {
			keyPath := filepath.Join(*keysDir, "namespace_key.json")
			if !*rotate {
				if data, err := os.ReadFile(keyPath); err == nil {
					var stored storedKey
					if json.Unmarshal(data, &stored) == nil {
						priv, err = crypto.ParsePrivateKeyHex(stored.PrivKeyHex)
						if err == nil {
							pubHex = stored.PubKeyXOnly
						}
					}
				}
			}

			// Generate new key if none exists or rotate requested
			if priv == nil {
				priv, pubHex, err = crypto.GenerateKeyPair()
				if err != nil {
					fmt.Fprintf(os.Stderr, "generate keypair: %v\n", err)
					os.Exit(1)
				}

				// Store the new key
				stored := storedKey{
					PrivKeyHex:    hex.EncodeToString(priv.Serialize()),
					PubKeyXOnly:   pubHex,
					CreatedAtUnix: time.Now().Unix(),
				}
				if err := os.MkdirAll(*keysDir, 0700); err != nil {
					fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", *keysDir, err)
					os.Exit(1)
				}
				if err := writeJSON0600(keyPath, stored); err != nil {
					fmt.Fprintf(os.Stderr, "write %s: %v\n", keyPath, err)
					os.Exit(1)
				}
			}
		}
	}

	// Create v0.2 Namespace Attestation
	payload := wire.NamespacePayload{
		Namespace: *namespace,
		Exp:       exp,
	}

	// Marshal to canonical JSON for signing
	payloadBytes, err := canonical.MarshalNamespacePayloadCanonical(payload.ToCanonical())
	if err != nil {
		fmt.Fprintf(os.Stderr, "canonical marshal: %v\n", err)
		os.Exit(1)
	}

	// Hash the payload
	digest := crypto.HashSHA256(payloadBytes)

	// Sign the digest
	sigHex, err := crypto.SignSchnorrHex(priv, digest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sign: %v\n", err)
		os.Exit(1)
	}

	// Create the full attestation object
	attestation := wire.NamespaceAttestation{
		Payload: payload,
		Key:     pubHex,
		Sig:     sigHex,
	}

	// Determine output directory and path
	outputDir := *out
	if outputDir == "" {
		outputDir = "."
	}
	
	// Create the full output path
	outputPath := filepath.Join(outputDir, "_la_namespace.json")

	// Create parent directory if it doesn't exist
	parentDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", parentDir, err)
		os.Exit(1)
	}

	// Write the attestation
	if err := writeJSON0600(outputPath, attestation); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", outputPath, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Created namespace attestation at %s\n", outputPath)
	fmt.Fprintf(os.Stderr, "Valid until %s\n", time.Unix(exp, 0).Format(time.RFC3339))
	fmt.Fprintf(os.Stderr, "Publisher key: %s\n", pubHex)
}

func writeJSON0600(path string, v any) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
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

// replaceArticleByDataLaFragmentURL finds the <article ...> element whose opening tag contains
// data-la-fragment-url="targetURL" and replaces the entire element with replacementHTML.
func replaceArticleByDataLaFragmentURL(hostHTML string, targetURL string, replacementHTML string) (string, bool) {
	needle := fmt.Sprintf("data-la-fragment-url=\"%s\"", targetURL)
	idx := strings.Index(hostHTML, needle)
	if idx < 0 {
		return hostHTML, false
	}
	start := strings.LastIndex(hostHTML[:idx], "<article")
	if start < 0 {
		return hostHTML, false
	}
	rest := hostHTML[start:]
	depth := 0
	i := 0
	for i < len(rest) {
		if rest[i] == '<' {
			if strings.HasPrefix(rest[i:], "<article") {
				depth++
			} else if strings.HasPrefix(rest[i:], "</article") {
				depth--
				endTag := strings.Index(rest[i:], ">")
				if endTag >= 0 {
					i += endTag + 1
				} else {
					break
				}
				if depth == 0 {
					endAbs := start + i
					var b strings.Builder
					b.WriteString(hostHTML[:start])
					b.WriteString(replacementHTML)
					b.WriteString(hostHTML[endAbs:])
					return b.String(), true
				}
				continue
			}
			end := strings.Index(rest[i:], ">")
			if end >= 0 {
				i += end + 1
				continue
			}
			break
		}
		i++
	}
	return hostHTML, false
}

// resetArtifactsCmd resets all LAP artifacts for alice by creating a new NA and updating all posts
func resetArtifactsCmd(args []string) {
	fs := flag.NewFlagSet("reset-artifacts", flag.ExitOnError)
	base := fs.String("base", "http://localhost:8081", "base URL (scheme://host[:port]) for LAP URLs")
	root := fs.String("root", "apps/server/static/publisherapi/people/alice", "root directory for Alice content")
	keysDir := fs.String("keys-dir", "keys", "directory containing publisher keys")
	_ = fs.Parse(args)

	// Load the publisher key from the keys directory
	aliceKeyPath := filepath.Join(*keysDir, "alice_publisher_key.json")
	
	var publisherKey string
	var privateKey string
	if data, err := os.ReadFile(aliceKeyPath); err == nil {
		var stored storedKey
		if json.Unmarshal(data, &stored) == nil && stored.PubKeyXOnly != "" {
			publisherKey = stored.PubKeyXOnly
			privateKey = stored.PrivKeyHex
		}
	}
	
	if publisherKey == "" || privateKey == "" {
		fmt.Fprintf(os.Stderr, "error: could not load publisher key from %s\n", aliceKeyPath)
		fmt.Fprintf(os.Stderr, "please create this key first using: lapctl keygen -name alice -out %s\n", aliceKeyPath)
		os.Exit(1)
	}

	// Step 1: Create new namespace attestation
	fmt.Fprintf(os.Stderr, "Creating new namespace attestation...\n")
	namespaceAttestationURL := fmt.Sprintf("%s/people/alice/_la_namespace.json", *base)
	
	// Use the existing naCreateCmd logic but with our specific parameters
	exp := time.Now().AddDate(1, 0, 0).Unix()
	
	// Parse private key
	priv, err := crypto.ParsePrivateKeyHex(privateKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse private key: %v\n", err)
		os.Exit(1)
	}
	
	// Create v0.2 Namespace Attestation
	payload := wire.NamespacePayload{
		Namespace: fmt.Sprintf("%s/people/alice/", *base),
		Exp:       exp,
	}

	// Marshal to canonical JSON for signing
	payloadBytes, err := canonical.MarshalNamespacePayloadCanonical(payload.ToCanonical())
	if err != nil {
		fmt.Fprintf(os.Stderr, "canonical marshal: %v\n", err)
		os.Exit(1)
	}

	// Hash the payload
	digest := crypto.HashSHA256(payloadBytes)

	// Sign the digest
	sigHex, err := crypto.SignSchnorrHex(priv, digest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sign: %v\n", err)
		os.Exit(1)
	}

	// Create the full attestation object
	attestation := wire.NamespaceAttestation{
		Payload: payload,
		Key:     publisherKey,
		Sig:     sigHex,
	}

	// Write the namespace attestation
	naOutputPath := filepath.Join(*root, "_la_namespace.json")
	if err := os.MkdirAll(filepath.Dir(naOutputPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", filepath.Dir(naOutputPath), err)
		os.Exit(1)
	}
	
	if err := writeJSON0600(naOutputPath, attestation); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", naOutputPath, err)
		os.Exit(1)
	}
	
	fmt.Fprintf(os.Stderr, "Created namespace attestation at %s\n", naOutputPath)
	fmt.Fprintf(os.Stderr, "Valid until %s\n", time.Unix(exp, 0).Format(time.RFC3339))

	// Step 2: Process each post (reusing existing logic from updatePostsCmd)
	fmt.Fprintf(os.Stderr, "Updating posts 1..3...\n")
	
	// Process each post
	for postNum := 1; postNum <= 3; postNum++ {
		postDir := filepath.Join(*root, "posts", strconv.Itoa(postNum))
		inPath := filepath.Join(postDir, "content.htmx")
		outPath := filepath.Join(postDir, "index.htmx")
		
		// Construct URLs for this post
		fragmentURL := fmt.Sprintf("%s/people/alice/posts/%d", *base, postNum)
		resourceAttestationURL := fmt.Sprintf("%s/people/alice/posts/%d/_la_resource.json", *base, postNum)
		
		// Generate resource attestation first
		fmt.Fprintf(os.Stderr, "generating resource attestation for post %d...\n", postNum)
		raArgs := []string{
			"ra-create",
			"-in", inPath,
			"-url", fragmentURL,
			"-publisher-claim", publisherKey,
			"-namespace-attestation-url", namespaceAttestationURL,
			"-out", filepath.Join(postDir, "_la_resource.json"),
		}
		
		cmd := exec.Command(os.Args[0], raArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "error generating RA for post %d: %v\n", postNum, err)
			os.Exit(1)
		}
		
		// Generate fragment
		fmt.Fprintf(os.Stderr, "generating fragment for post %d...\n", postNum)
		fragmentArgs := []string{
			"fragment-create",
			"-in", inPath,
			"-url", fragmentURL,
			"-publisher-claim", publisherKey,
			"-resource-attestation-url", resourceAttestationURL,
			"-namespace-attestation-url", namespaceAttestationURL,
			"-out", outPath,
		}
		
		cmd = exec.Command(os.Args[0], fragmentArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "error generating fragment for post %d: %v\n", postNum, err)
			os.Exit(1)
		}
	}
	
	// Step 3: Update the host file with all three fragments
	hostPath := filepath.Join(*root, "index.html")
	if _, err := os.Stat(hostPath); err == nil {
		fmt.Fprintf(os.Stderr, "updating host file %s...\n", hostPath)
		
		hostHTML, err := os.ReadFile(hostPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read host file: %v\n", err)
			os.Exit(1)
		}
		
		// Read each fragment and update the host file
		for postNum := 1; postNum <= 3; postNum++ {
			fragmentPath := filepath.Join(*root, "posts", strconv.Itoa(postNum), "index.htmx")
			fragmentData, err := os.ReadFile(fragmentPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "read fragment %d: %v\n", postNum, err)
				os.Exit(1)
			}
			
			// The fragment is HTML, not JSON, so we can use it directly
			replacementHTML := string(fragmentData)
			
			// Update host file
			fragmentURL := fmt.Sprintf("%s/people/alice/posts/%d", *base, postNum)
			updatedHTML, updated := replaceArticleByDataLaFragmentURL(string(hostHTML), fragmentURL, replacementHTML)
			if updated {
				hostHTML = []byte(updatedHTML)
				fmt.Fprintf(os.Stderr, "updated post %d in host file\n", postNum)
			} else {
				fmt.Fprintf(os.Stderr, "warning: could not find post %d in host file\n", postNum)
			}
		}
		
		// Write updated host file
		if err := os.WriteFile(hostPath, hostHTML, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "write host file: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Fprintf(os.Stderr, "successfully updated host file %s\n", hostPath)
	} else {
		fmt.Fprintf(os.Stderr, "host file %s not found, skipping host update\n", hostPath)
	}
	
	fmt.Fprintf(os.Stderr, "Successfully reset all LAP artifacts for alice\n")
}
