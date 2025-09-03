// Package artifacts provides demo utilities for LAP artifact management.
package artifacts

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// CreateFragment creates a v0.2 HTML fragment from the given content
func CreateFragment(inPath, resURL, base, publisherClaim, resourceAttestationURL, namespaceAttestationURL, outPath string) error {
	// Read input file
	body, err := os.ReadFile(inPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", inPath, err)
	}

	// Build payload URL with optional base override
	var u url.URL
	if base != "" {
		baseURL, err := url.Parse(base)
		if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
			return fmt.Errorf("invalid base: %s", base)
		}
		u = *baseURL
		// take path+query from resURL (absolute or relative)
		rawU, err := url.Parse(resURL)
		if err != nil {
			return fmt.Errorf("invalid url: %s", resURL)
		}
		if rawU.Path != "" {
			u.Path = rawU.Path
		}
		u.RawQuery = rawU.RawQuery
	} else {
		rawU, err := url.Parse(resURL)
		if err != nil || rawU.Scheme == "" || rawU.Host == "" {
			return fmt.Errorf("invalid url (expect absolute when base not set): %s", resURL)
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

	// Indent the content body to match the fragment structure
	indentedBody := indentContent(string(body), "    ")

	article := "" +
		"<article\n" +
		"  data-la-spec=\"v0.2\"\n" +
		fmt.Sprintf("  data-la-fragment-url=\"%s\"\n", payloadURL) +
		">\n" +
		"  <section class=\"la-preview\">\n" +
		indentedBody + "\n" +
		"  </section>\n" +
		"  <link\n" +
		"    rel=\"canonical\"\n" +
		"    type=\"text/html\"\n" +
		fmt.Sprintf("    data-la-publisher-claim=\"%s\"\n", publisherClaim) +
		fmt.Sprintf("    data-la-resource-attestation-url=\"%s\"\n", resourceAttestationURL) +
		fmt.Sprintf("    data-la-namespace-attestation-url=\"%s\"\n", namespaceAttestationURL) +
		fmt.Sprintf("    href=\"data:text/html;base64,%s\"\n", b64) +
		"    hidden\n" +
		"  />\n" +
		"</article>"

	// Determine output path
	if outPath == "" {
		outPath = filepath.Join(filepath.Dir(inPath), "index.htmx")
	}

	// Create output directory and file
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	
	return os.WriteFile(outPath, []byte(article), 0644)
}

// UpdateHostFile updates a host HTML file with a new fragment and formats it
func UpdateHostFile(hostPath, fragmentURL, fragmentHTML string) error {
	hostBytes, err := os.ReadFile(hostPath)
	if err != nil {
		return fmt.Errorf("read host file %s: %w", hostPath, err)
	}
	
	replaced, ok := ReplaceArticleByDataLaFragmentURL(string(hostBytes), fragmentURL, fragmentHTML)
	if !ok {
		return fmt.Errorf("did not find <article> with data-la-fragment-url=\"%s\" in %s", fragmentURL, hostPath)
	}
	
	// Add consistent spacing between fragments for better readability
	formattedHTML := addFragmentSpacing(replaced)
	
	// Create backup
	if err := os.WriteFile(hostPath+".bak", hostBytes, 0644); err != nil {
		return fmt.Errorf("backup %s: %w", hostPath+".bak", err)
	}
	
	// Write updated file
	return os.WriteFile(hostPath, []byte(formattedHTML), 0644)
}

// indentContent adds the specified indentation to each line of the content
func indentContent(content, indent string) string {
	lines := strings.Split(content, "\n")
	var indentedLines []string
	
	for i, line := range lines {
		// Don't add indentation to empty lines
		if strings.TrimSpace(line) == "" {
			indentedLines = append(indentedLines, line)
		} else {
			indentedLines = append(indentedLines, indent+line)
		}
		
		// Don't add trailing newline for the last line if it's empty
		if i == len(lines)-1 && strings.TrimSpace(line) == "" {
			continue
		}
	}
	
	return strings.Join(indentedLines, "\n")
}

// addFragmentSpacing ensures consistent spacing between fragments in the host file
func addFragmentSpacing(html string) string {
	lines := strings.Split(html, "\n")
	var formattedLines []string
	emptyLineCount := 0
	
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		
		// Handle empty lines - limit consecutive empty lines to 1
		if trimmed == "" {
			emptyLineCount++
			if emptyLineCount <= 1 {
				formattedLines = append(formattedLines, "")
			}
			continue
		}
		
		// Reset empty line counter when we hit non-empty content
		emptyLineCount = 0
		formattedLines = append(formattedLines, line)
	}
	
	// Remove trailing empty lines
	for len(formattedLines) > 0 && strings.TrimSpace(formattedLines[len(formattedLines)-1]) == "" {
		formattedLines = formattedLines[:len(formattedLines)-1]
	}
	
	return strings.Join(formattedLines, "\n")
}
