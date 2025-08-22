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
	fmt.Fprintf(os.Stderr, "  ra-create   Create a resource attestation for an HTML file\n")
	fmt.Fprintf(os.Stderr, "  fragment-create   Create an HTML fragment (index.htmx) and matching RA from an index.html\n")
	fmt.Fprintf(os.Stderr, "  update-posts      Generate fragments for posts 1..3 with independent window-min values\n")
	fmt.Fprintf(os.Stderr, "  na-create     Create a namespace attestation for a namespace URL\n")
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
	KID           string `json:"kid"`
	PrivKeyHex    string `json:"privkey_hex"`
	PubKeyXOnly   string `json:"pubkey_xonly_hex"`
	CreatedAtUnix int64  `json:"created_at"`
}

// parseWindowToSeconds parses strings like "30s", "5m", "2h", "1d" or plain minutes (e.g. "10") into seconds
func parseWindowToSeconds(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty window")
	}
	unit := s[len(s)-1]
	numStr := s
	switch unit {
	case 's', 'S':
		numStr = s[:len(s)-1]
		v, err := strconv.Atoi(numStr)
		if err != nil || v < 0 {
			return 0, fmt.Errorf("invalid seconds: %s", s)
		}
		return v, nil
	case 'm', 'M':
		numStr = s[:len(s)-1]
		v, err := strconv.Atoi(numStr)
		if err != nil || v < 0 {
			return 0, fmt.Errorf("invalid minutes: %s", s)
		}
		return v * 60, nil
	case 'h', 'H':
		numStr = s[:len(s)-1]
		v, err := strconv.Atoi(numStr)
		if err != nil || v < 0 {
			return 0, fmt.Errorf("invalid hours: %s", s)
		}
		return v * 60 * 60, nil
	case 'd', 'D':
		numStr = s[:len(s)-1]
		v, err := strconv.Atoi(numStr)
		if err != nil || v < 0 {
			return 0, fmt.Errorf("invalid days: %s", s)
		}
		return v * 24 * 60 * 60, nil
	default:
		// plain number -> minutes
		v, err := strconv.Atoi(s)
		if err != nil || v < 0 {
			return 0, fmt.Errorf("invalid window: %s", s)
		}
		return v * 60, nil
	}
}

func raCreateCmd(args []string) {
	fs := flag.NewFlagSet("ra-create", flag.ExitOnError)
	inPath := fs.String("in", "", "path to input HTML file")
	resURL := fs.String("url", "", "absolute resource URL or path (e.g. https://example.com/path or /people/alice/posts/1)")
	base := fs.String("base", "", "optional base (scheme://host[:port]) to resolve -url against, e.g. http://localhost:8081")
	kid := fs.String("kid", "", "key identifier for this resource")
	privHexFlag := fs.String("privkey", "", "(optional) hex-encoded resource private key; if provided, will be used and stored")
	etagFlag := fs.String("etag", "", "optional ETag value; if empty, computed as W/\"<sha256hex>\"")
	// Timebox flags: prefer window-min if provided; fallback to ttl seconds; else default 10 minutes
	expSeconds := fs.Int("ttl", -1, "(optional) seconds until expiration; deprecated, prefer -window-min")
	windowStr := fs.String("window-min", "10m", "freshness window (e.g. 30s, 5m, 1d). Plain numbers are minutes")
	out := fs.String("out", "", "output file path (default: <dir>/_lap/resource_attestation.json)")
	keysDir := fs.String("keys-dir", "keys", "directory to store per-resource keys (outside static)")
	rotate := fs.Bool("rotate", false, "force generating a new keypair even if one exists for this resource")
	_ = fs.Parse(args)

	if *inPath == "" || *resURL == "" || *kid == "" {
		fmt.Fprintf(os.Stderr, "ra-create requires -in, -url, and -kid\n")
		fs.Usage()
		os.Exit(2)
	}

	body, err := os.ReadFile(*inPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", *inPath, err)
		os.Exit(1)
	}

	// Compute or use ETag
	etag := *etagFlag
	if etag == "" {
		etag = fmt.Sprintf("W/\"%s\"", crypto.HashSHA256Hex(body))
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

	// attestation_url: same origin, path (strip trailing /index.html or /index.json), then add /_lap/resource_attestation.json; no query/fragment
	uAtt := u
	uAtt.RawQuery = ""
	uAtt.Fragment = ""
	p := uAtt.Path
	p = strings.TrimSuffix(p, "/index.html")
	p = strings.TrimSuffix(p, "/index.json")
	p = strings.TrimSuffix(p, "/")
	uAtt.Path = p + "/_lap/resource_attestation.json"
	attestationURL := uAtt.String()

	// Keys: reuse or generate per-resource key stored under keysDir mirroring the input path
	relIn := *inPath
	if absIn, err := filepath.Abs(*inPath); err == nil {
		relIn = strings.TrimPrefix(absIn, string(filepath.Separator))
	}
	keyPath := filepath.Join(*keysDir, relIn, "resource_key.json")

	var privKey *btcec.PrivateKey
	var privHex, pubHex string

	// Try load existing unless rotate
	if !*rotate {
		if data, err := os.ReadFile(keyPath); err == nil {
			var k storedKey
			if json.Unmarshal(data, &k) == nil && k.PrivKeyHex != "" {
				if pk, err := crypto.ParsePrivateKeyHex(k.PrivKeyHex); err == nil {
					privKey = pk
					privHex = k.PrivKeyHex
					pubHex = k.PubKeyXOnly
				}
			}
		}
	}

	// If provided via flag, override
	if privKey == nil && *privHexFlag != "" {
		pk, err := crypto.ParsePrivateKeyHex(*privHexFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid -privkey: %v\n", err)
			os.Exit(2)
		}
		privKey = pk
		privHex = hex.EncodeToString(pk.Serialize())
		// Derive x-only pub
		xonly := schnorr.SerializePubKey(pk.PubKey())
		pubHex = hex.EncodeToString(xonly)
	}

	// Generate if still nil
	if privKey == nil {
		pk, pub, err := crypto.GenerateKeyPair()
		if err != nil {
			fmt.Fprintf(os.Stderr, "keygen error: %v\n", err)
			os.Exit(1)
		}
		privKey = pk
		privHex = hex.EncodeToString(pk.Serialize())
		pubHex = pub
	}

	// Persist key (mkdir -p, 0600 file)
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err == nil {
		_ = writeJSON0600(keyPath, storedKey{
			KID:           *kid,
			PrivKeyHex:    privHex,
			PubKeyXOnly:   pubHex,
			CreatedAtUnix: time.Now().UTC().Unix(),
		})
	}

	// Build payload and sign
	now := time.Now().UTC().Unix()
	var exp int64
	if *windowStr != "" {
		secs, err := parseWindowToSeconds(*windowStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid -window-min: %v\n", err)
			os.Exit(2)
		}
		exp = now + int64(secs)
	} else if *expSeconds >= 0 {
		exp = now + int64(*expSeconds)
	} else {
		exp = now + 10*60 // default 10 minutes
	}
	payload := wire.Payload{
		URL:             payloadURL,
		Attestation_URL: attestationURL,
		Hash:            crypto.ComputeContentHashField(body),
		ETag:            etag,
		IAT:             now,
		EXP:             exp,
		KID:             *kid,
	}
	bytesPayload, err := canonical.MarshalPayloadCanonical(payload.ToCanonical())
	if err != nil {
		fmt.Fprintf(os.Stderr, "canonical marshal: %v\n", err)
		os.Exit(1)
	}
	digest := crypto.HashSHA256(bytesPayload)
	sigHex, err := crypto.SignSchnorrHex(privKey, digest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sign: %v\n", err)
		os.Exit(1)
	}
	att := wire.Attestation{Payload: payload, ResourceKey: pubHex, Sig: sigHex}

	// Output
	outPath := *out
	if outPath == "" {
		dir := filepath.Dir(*inPath)
		outPath = filepath.Join(dir, "_lap", "resource_attestation.json")
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
	
	// Write the exact canonical payload bytes that were signed
	bytesPath := filepath.Join(filepath.Dir(outPath), "bytes.txt")
	if err := os.WriteFile(bytesPath, bytesPayload, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", bytesPath, err)
		os.Exit(1)
	}
	
	fmt.Fprintf(os.Stderr, "wrote %s\n", outPath)
	fmt.Fprintf(os.Stderr, "wrote %s\n", bytesPath)
}

func fragmentCreateCmd(args []string) {
	fs := flag.NewFlagSet("fragment-create", flag.ExitOnError)
	inPath := fs.String("in", "", "path to input index.html file")
	resURL := fs.String("url", "", "absolute resource URL or path (e.g. https://example.com/path or /people/alice/messages/1)")
	base := fs.String("base", "", "optional base (scheme://host[:port]) to resolve -url against, e.g. http://localhost:8081")
	kid := fs.String("kid", "", "key identifier for this resource")
	privHexFlag := fs.String("privkey", "", "(optional) hex-encoded resource private key; if provided, will be used and stored")
	etagFlag := fs.String("etag", "", "optional ETag value; if empty, computed as W/\"<sha256hex>\"")
	// Timebox flags: prefer window (duration) if provided; fallback to window-min minutes, then ttl seconds
	expSeconds := fs.Int("ttl", -1, "(optional) seconds until expiration; deprecated, prefer -window")
	windowMin := fs.Int("window-min", -1, "(deprecated) freshness window in minutes")
	windowStr := fs.String("window", "10m", "freshness window (e.g. 30s, 5m, 1d). Plain numbers are minutes")
	out := fs.String("out", "", "output fragment HTML path (default: <dir>/index.htmx)")
	updateHost := fs.String("update", "", "optional path to host HTML file whose matching <article data-lap-url> should be replaced with the new fragment")
	dryRun := fs.Bool("dry-run", false, "if set, do not write changes to -update host file; just report action")
	keysDir := fs.String("keys-dir", "keys", "directory to store per-resource keys (outside static)")
	rotate := fs.Bool("rotate", false, "force generating a new keypair even if one exists for this resource")
	_ = fs.Parse(args)

	if *inPath == "" || *resURL == "" || *kid == "" {
		fmt.Fprintf(os.Stderr, "fragment-create requires -in, -url, and -kid\n")
		fs.Usage()
		os.Exit(2)
	}

	body, err := os.ReadFile(*inPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", *inPath, err)
		os.Exit(1)
	}

	// Compute or use ETag (over the canonical body bytes)
	etag := *etagFlag
	if etag == "" {
		etag = fmt.Sprintf("W/\"%s\"", crypto.HashSHA256Hex(body))
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

	// attestation_url: same origin, path (strip trailing /index.html or /index.json), then add /_lap/resource_attestation.json; no query/fragment
	uAtt := u
	uAtt.RawQuery = ""
	uAtt.Fragment = ""
	p := uAtt.Path
	p = strings.TrimSuffix(p, "/index.html")
	p = strings.TrimSuffix(p, "/index.json")
	p = strings.TrimSuffix(p, "/")
	uAtt.Path = p + "/_lap/resource_attestation.json"
	attestationURL := uAtt.String()

	// Keys: reuse or generate per-resource key stored under keysDir mirroring the input path
	relIn := *inPath
	if absIn, err := filepath.Abs(*inPath); err == nil {
		relIn = strings.TrimPrefix(absIn, string(filepath.Separator))
	}
	keyPath := filepath.Join(*keysDir, relIn, "resource_key.json")

	var privKey *btcec.PrivateKey
	var privHex, pubHex string

	// Try load existing unless rotate
	if !*rotate {
		if data, err := os.ReadFile(keyPath); err == nil {
			var k storedKey
			if json.Unmarshal(data, &k) == nil && k.PrivKeyHex != "" {
				if pk, err := crypto.ParsePrivateKeyHex(k.PrivKeyHex); err == nil {
					privKey = pk
					privHex = k.PrivKeyHex
					pubHex = k.PubKeyXOnly
				}
			}
		}
	}

	// If provided via flag, override
	if privKey == nil && *privHexFlag != "" {
		pk, err := crypto.ParsePrivateKeyHex(*privHexFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid -privkey: %v\n", err)
			os.Exit(2)
		}
		privKey = pk
		privHex = hex.EncodeToString(pk.Serialize())
		// Derive x-only pub
		xonly := schnorr.SerializePubKey(pk.PubKey())
		pubHex = hex.EncodeToString(xonly)
	}

	// Generate if still nil
	if privKey == nil {
		pk, pub, err := crypto.GenerateKeyPair()
		if err != nil {
			fmt.Fprintf(os.Stderr, "keygen error: %v\n", err)
			os.Exit(1)
		}
		privKey = pk
		privHex = hex.EncodeToString(pk.Serialize())
		pubHex = pub
	}

	// Persist key (mkdir -p, 0600 file)
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err == nil {
		_ = writeJSON0600(keyPath, storedKey{
			KID:           *kid,
			PrivKeyHex:    privHex,
			PubKeyXOnly:   pubHex,
			CreatedAtUnix: time.Now().UTC().Unix(),
		})
	}

	// Build payload and sign
	now := time.Now().UTC().Unix()
	var exp int64
	if *windowStr != "" {
		secs, err := parseWindowToSeconds(*windowStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid -window: %v\n", err)
			os.Exit(2)
		}
		exp = now + int64(secs)
	} else if *windowMin >= 0 {
		exp = now + int64(*windowMin)*60
	} else if *expSeconds >= 0 {
		exp = now + int64(*expSeconds)
	} else {
		exp = now + 10*60 // default 10 minutes
	}
	payload := wire.Payload{
		URL:             payloadURL,
		Attestation_URL: attestationURL,
		Hash:            crypto.ComputeContentHashField(body),
		ETag:            etag,
		IAT:             now,
		EXP:             exp,
		KID:             *kid,
	}
	bytesPayload, err := canonical.MarshalPayloadCanonical(payload.ToCanonical())
	if err != nil {
		fmt.Fprintf(os.Stderr, "canonical marshal: %v\n", err)
		os.Exit(1)
	}
	digest := crypto.HashSHA256(bytesPayload)
	sigHex, err := crypto.SignSchnorrHex(privKey, digest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sign: %v\n", err)
		os.Exit(1)
	}
	att := wire.Attestation{Payload: payload, ResourceKey: pubHex, Sig: sigHex}

	// Write RA JSON alongside (default: <dir>/_lap/resource_attestation.json)
	raOut := filepath.Join(filepath.Dir(*inPath), "_lap", "resource_attestation.json")
	if err := os.MkdirAll(filepath.Dir(raOut), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	{
		f, err := os.Create(raOut)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create %s: %v\n", raOut, err)
			os.Exit(1)
		}
		enc := json.NewEncoder(f)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(att); err != nil {
			_ = f.Close()
			fmt.Fprintf(os.Stderr, "write json: %v\n", err)
			os.Exit(1)
		}
		_ = f.Close()
	}

	// Write the exact canonical payload bytes that were signed
	bytesOut := filepath.Join(filepath.Dir(*inPath), "_lap", "bytes.txt")
	if err := os.WriteFile(bytesOut, bytesPayload, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", bytesOut, err)
		os.Exit(1)
	}

	// Build fragment HTML (DIV-format attestation + link-data bytes)
	// Base64 of the exact canonical body bytes
	b64 := base64.StdEncoding.EncodeToString(body)

	article := "" +
		"<article id=\"lap-article\"\n" +
		"  data-lap-spec=\"https://lap.dev/spec/v0-1\"\n" +
		"  data-lap-profile=\"fragment\"\n" +
		"  data-lap-attestation-format=\"div\"\n" +
		"  data-lap-bytes-format=\"link-data\"\n\n" +
		fmt.Sprintf("  data-lap-url=\"%s\"\n", payloadURL) +
		"  data-lap-preview=\"#lap-preview\"\n" +
		"  data-lap-attestation=\"#lap-attestation\"\n" +
		"  data-lap-bytes=\"#lap-bytes\">\n\n" +
		"  <div id=\"lap-preview\" class=\"lap-preview\">\n" +
		string(body) + "\n" +
		"  </div>\n\n" +
		fmt.Sprintf("  <link id=\"lap-bytes\" rel=\"alternate\" type=\"text/html; charset=utf-8\" class=\"lap-bytes\" data-hash=\"%s\" href=\"data:text/html;base64,%s\">\n\n", payload.Hash, b64) +
		fmt.Sprintf("  <div id=\"lap-attestation\" class=\"lap-attestation\" data-lap-resource-key=\"%s\" data-lap-sig=\"%s\" hidden>\n", att.ResourceKey, att.Sig) +
		fmt.Sprintf("    <div class=\"lap-payload\" data-lap-url=\"%s\" data-lap-attestation-url=\"%s\" data-lap-hash=\"%s\" data-lap-etag='"+"%s"+"' data-lap-iat=\"%d\" data-lap-exp=\"%d\" data-lap-kid=\"%s\"></div>\n", payload.URL, payload.Attestation_URL, payload.Hash, payload.ETag, payload.IAT, payload.EXP, payload.KID) +
		"  </div>\n" +
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
	fmt.Fprintf(os.Stderr, "wrote %s\n", raOut)
	fmt.Fprintf(os.Stderr, "wrote %s\n", bytesOut)

	if *updateHost != "" {
		hostBytes, err := os.ReadFile(*updateHost)
		if err != nil {
			fmt.Fprintf(os.Stderr, "update read %s: %v\n", *updateHost, err)
			os.Exit(1)
		}
		replaced, ok := replaceArticleByDataLapURL(string(hostBytes), payloadURL, article)
		if !ok {
			fmt.Fprintf(os.Stderr, "update: did not find <article> with data-lap-url=\"%s\" in %s\n", payloadURL, *updateHost)
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

// updatePostsCmd runs fragment-create for posts 1..3, allowing distinct -window-min values per post.
// Usage:
//   lapctl update-posts -base http://localhost:8081 -dir apps/server/static/publisherapi/people/alice \
//     -w1 3 -w2 5 -w3 10 -kid-prefix key-post-
func updatePostsCmd(args []string) {
    fs := flag.NewFlagSet("update-posts", flag.ExitOnError)
    base := fs.String("base", "http://localhost:8081", "base (scheme://host[:port]) for -url resolution")
    root := fs.String("dir", "apps/server/static/publisherapi/people/alice", "root directory for Alice content")
    host := fs.String("update", "apps/server/static/publisherapi/people/alice/index.html", "host HTML file to update (list page)")
    kidPrefix := fs.String("kid-prefix", "key-post-", "prefix for -kid values (post number appended)")
    w1 := fs.String("w1", "3m", "freshness window (e.g. 30s, 5m, 1d) for post 1")
    w2 := fs.String("w2", "5m", "freshness window (e.g. 30s, 5m, 1d) for post 2")
    w3 := fs.String("w3", "10m", "freshness window (e.g. 30s, 5m, 1d) for post 3")
    _ = fs.Parse(args)

    // Validate provided durations (accept s/m/d or plain minutes)
    if _, err := parseWindowToSeconds(*w1); err != nil {
        fmt.Fprintf(os.Stderr, "invalid -w1: %v\n", err)
        os.Exit(2)
    }
    if _, err := parseWindowToSeconds(*w2); err != nil {
        fmt.Fprintf(os.Stderr, "invalid -w2: %v\n", err)
        os.Exit(2)
    }
    if _, err := parseWindowToSeconds(*w3); err != nil {
        fmt.Fprintf(os.Stderr, "invalid -w3: %v\n", err)
        os.Exit(2)
    }

    type job struct{ post int; winStr string }
    jobs := []job{{1, *w1}, {2, *w2}, {3, *w3}}

    for _, j := range jobs {
        in := filepath.Join(*root, "posts", strconv.Itoa(j.post), "index.html")
        out := filepath.Join(*root, "posts", strconv.Itoa(j.post), "index.htmx")
        urlPath := fmt.Sprintf("/people/alice/posts/%d", j.post)
        kid := fmt.Sprintf("%s%d", *kidPrefix, j.post)

        args := []string{
            "fragment-create",
            "-in", in,
            "-url", urlPath,
            "-base", *base,
            "-window", j.winStr,
            "-kid", kid,
            "-out", out,
            "-update", *host,
        }
        fmt.Fprintf(os.Stderr, "running: lapctl %s\n", strings.Join(args, " "))
        cmd := exec.Command(os.Args[0], args...)
        cmd.Stdout = os.Stdout
        cmd.Stderr = os.Stderr
        if err := cmd.Run(); err != nil {
            fmt.Fprintf(os.Stderr, "error: %v\n", err)
            os.Exit(1)
        }
    }
}

func naCreateCmd(args []string) {
	fs := flag.NewFlagSet("na-create", flag.ExitOnError)
	namespace := fs.String("namespace", "", "namespace URL (e.g. https://example.com/people/alice/)")
	windowStr := fs.String("window", "1d", "freshness window (e.g. 30s, 5m, 2h, 1d). Plain numbers are minutes")
	kid := fs.String("kid", "", "key identifier for this namespace attestation")
	privHexFlag := fs.String("privkey", "", "(optional) hex-encoded publisher private key; if provided, will be used and stored")
	out := fs.String("out", "", "output directory path (default: current directory)")
	keysDir := fs.String("keys-dir", "keys", "directory to store per-namespace keys (outside static)")
	rotate := fs.Bool("rotate", false, "force generating a new keypair even if one exists for this namespace")
	_ = fs.Parse(args)

	if *namespace == "" || *kid == "" {
		fmt.Fprintf(os.Stderr, "na-create requires -namespace and -kid\n")
		fs.Usage()
		os.Exit(2)
	}

	// Parse window duration
	windowSeconds, err := parseWindowToSeconds(*windowStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid -window: %v\n", err)
		os.Exit(2)
	}

	// Calculate timestamps
	now := time.Now()
	iat := now.Unix()
	exp := now.Add(time.Duration(windowSeconds) * time.Second).Unix()

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
		// Try to load existing key from keys directory
		keyPath := filepath.Join(*keysDir, *kid+".json")
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
				KID:           *kid,
				PrivKeyHex:    hex.EncodeToString(priv.Serialize()),
				PubKeyXOnly:   pubHex,
				CreatedAtUnix: now.Unix(),
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

	// Create canonical payload
	payload := map[string]interface{}{
		"namespace":        []string{*namespace},
		"attestation_path": "_lap/namespace_attestation.json",
		"iat":              iat,
		"exp":              exp,
		"kid":              *kid,
	}

	// Canonicalize the payload (sort fields in fixed order)
	canonicalPayload := map[string]interface{}{
		"namespace":        payload["namespace"],
		"attestation_path": payload["attestation_path"],
		"iat":              payload["iat"],
		"exp":              payload["exp"],
		"kid":              payload["kid"],
	}

	// Marshal to JSON for signing
	payloadBytes, err := json.Marshal(canonicalPayload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal payload: %v\n", err)
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
	attestation := map[string]interface{}{
		"payload":      canonicalPayload,
		"publisher_key": pubHex,
		"sig":          sigHex,
	}

	// Determine output path
	outputDir := *out
	if outputDir == "" {
		outputDir = "."
	}

	// Create _lap directory if it doesn't exist
	lapDir := filepath.Join(outputDir, "_lap")
	if err := os.MkdirAll(lapDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s: %v\n", lapDir, err)
		os.Exit(1)
	}

	// Write the attestation
	outputPath := filepath.Join(lapDir, "namespace_attestation.json")
	if err := writeJSON0600(outputPath, attestation); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", outputPath, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Created namespace attestation at %s\n", outputPath)
	fmt.Fprintf(os.Stderr, "Valid from %s to %s\n", time.Unix(iat, 0).Format(time.RFC3339), time.Unix(exp, 0).Format(time.RFC3339))
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

// replaceArticleByDataLapURL finds the <article ...> element whose opening tag contains
// data-lap-url="targetURL" and replaces the entire element with replacementHTML.
func replaceArticleByDataLapURL(hostHTML string, targetURL string, replacementHTML string) (string, bool) {
	needle := fmt.Sprintf("data-lap-url=\"%s\"", targetURL)
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
