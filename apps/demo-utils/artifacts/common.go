// Package artifacts provides demo utilities for LAP artifact management.
// This package contains shared types and utility functions used across
// multiple artifact operations.
package artifacts

import (
	"encoding/json"
	"os"
	"strings"
)

// StoredKey represents a stored key pair in JSON format
type StoredKey struct {
	PrivKeyHex    string `json:"privkey_hex"`
	PubKeyXOnly   string `json:"pubkey_xonly_hex"`
	CreatedAtUnix int64  `json:"created_at"`
}

// WriteJSON0600 writes a value as JSON to a file with 0600 permissions and proper formatting
func WriteJSON0600(path string, v any) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)  // Don't escape HTML characters
	enc.SetIndent("", "  ")   // Pretty print with 2-space indentation
	return enc.Encode(v)
}

// ReplaceArticleByDataLaFragmentURL finds the <article ...> element whose opening tag contains
// data-la-fragment-url="targetURL" and replaces the entire element with replacementHTML.
func ReplaceArticleByDataLaFragmentURL(hostHTML string, targetURL string, replacementHTML string) (string, bool) {
	needle := "data-la-fragment-url=\"" + targetURL + "\""
	idx := strings.Index(hostHTML, needle)
	if idx < 0 {
		return hostHTML, false
	}

	// Find the start of the article element
	start := strings.LastIndex(hostHTML[:idx], "<article")
	if start < 0 {
		return hostHTML, false
	}

	// Find the end of the article element
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
					return hostHTML[:start] + replacementHTML + hostHTML[endAbs:], true
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
