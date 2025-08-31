package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stonebraker/lap/sdks/go/pkg/lap/wire"
)

// Test helper functions

func setupTestEnvironment(t *testing.T) (string, func()) {
	tmpDir := t.TempDir()
	
	// Create a test keys directory
	keysDir := filepath.Join(tmpDir, "keys")
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		t.Fatalf("Failed to create keys directory: %v", err)
	}
	
	cleanup := func() {
		// t.TempDir() automatically cleans up, but we can add any additional cleanup here
	}
	
	return tmpDir, cleanup
}

func runLapctl(t *testing.T, args ...string) (string, string, error) {
	// Get the test file's directory and navigate to project root
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("Failed to get test file location")
	}
	
	testDir := filepath.Dir(testFile)
	projectRoot := filepath.Join(testDir, "../../../..")
	lapctlPath := filepath.Join(projectRoot, "bin", "lapctl")
	
	// Verify the binary exists
	if _, err := os.Stat(lapctlPath); os.IsNotExist(err) {
		t.Fatalf("lapctl binary not found at %s. Please run 'go build -o bin/lapctl ./apps/tools-cli/cmd/lapctl' from the project root", lapctlPath)
	}
	
	// Run the command
	cmd := exec.Command(lapctlPath, args...)
	cmd.Dir = "."
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", string(output), err
	}
	
	return string(output), "", nil
}

func readNamespaceAttestation(t *testing.T, filePath string) *wire.NamespaceAttestation {
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read attestation file: %v", err)
	}
	
	var attestation wire.NamespaceAttestation
	if err := json.Unmarshal(data, &attestation); err != nil {
		t.Fatalf("Failed to unmarshal attestation: %v", err)
	}
	
	return &attestation
}

func readResourceAttestation(t *testing.T, filePath string) *wire.ResourceAttestation {
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read resource attestation file: %v", err)
	}
	
	var attestation wire.ResourceAttestation
	if err := json.Unmarshal(data, &attestation); err != nil {
		t.Fatalf("Failed to unmarshal resource attestation: %v", err)
	}
	
	return &attestation
}

// Test cases

func TestNaCreate_DefaultBehavior(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()
	
	// Change to temp directory for test
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)
	
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	
	// Run na-create with minimal arguments
	output, stderr, err := runLapctl(t, "na-create", 
		"-namespace", "https://example.com/people/alice/")
	
	if err != nil {
		t.Fatalf("na-create failed: %v\nstderr: %s", err, stderr)
	}
	
	// Verify output contains expected messages
	if output == "" {
		t.Error("Expected output, got empty string")
	}
	
	// Check that the default file was created (using the default path from the command)
	expectedPath := "_la_namespace.json"
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected file %s was not created", expectedPath)
	}
	
	// Verify the attestation file content
	attestation := readNamespaceAttestation(t, expectedPath)
	
	// Check required fields
	if attestation.Payload.Namespace != "https://example.com/people/alice/" {
		t.Errorf("Expected namespace %s, got %s", 
			"https://example.com/people/alice/", 
			attestation.Payload.Namespace)
	}
	
	// Check timestamps
	now := time.Now().Unix()
	if attestation.Payload.Exp == 0 {
		t.Error("Expected Exp to be set")
	}
	if attestation.Payload.Exp <= now {
		t.Error("Expected Exp to be in the future")
	}
	// Verify timestamps are reasonable (not in the distant past/future)
	if attestation.Payload.Exp < now || attestation.Payload.Exp > now+86400*365 {
		t.Errorf("Expected Exp to be reasonable, got %d (now: %d)", attestation.Payload.Exp, now)
	}
	
	// Check signature fields
	if attestation.Key == "" {
		t.Error("Expected key to be set")
	}
	if attestation.Sig == "" {
		t.Error("Expected sig to be set")
	}
}

func TestNaCreate_CustomOutputPath(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()
	
	// Change to temp directory for test
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)
	
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	
	// Run na-create with custom output path
	output, stderr, err := runLapctl(t, "na-create",
		"-namespace", "https://example.com/people/bob/",
		"-out", "custom-output")
	
	if err != nil {
		t.Fatalf("na-create failed: %v\nstderr: %s", err, stderr)
	}
	
	// Verify output contains expected messages
	if output == "" {
		t.Error("Expected output, got empty string")
	}
	
	// Check that the custom file was created
	expectedPath := filepath.Join("custom-output", "_la_namespace.json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected file %s was not created", expectedPath)
	}
	
	// Verify the attestation file content
	attestation := readNamespaceAttestation(t, expectedPath)
	
	// Check namespace
	if attestation.Payload.Namespace != "https://example.com/people/bob/" {
		t.Errorf("Expected namespace %s, got %s",
			"https://example.com/people/bob/",
			attestation.Payload.Namespace)
	}
}

func TestNaCreate_CustomExpiration(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()
	
	// Change to temp directory for test
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)
	
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	
	// Run na-create with custom expiration
	output, stderr, err := runLapctl(t, "na-create",
		"-namespace", "https://example.com/people/charlie/",
		"-exp", "1641003600")
	
	if err != nil {
		t.Fatalf("na-create failed: %v\nstderr: %s", err, stderr)
	}
	
	// Verify output contains expected messages
	if output == "" {
		t.Error("Expected output, got empty string")
	}
	
	// Check that the file was created (using the default path)
	expectedPath := "_la_namespace.json"
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected file %s was not created", expectedPath)
	}
	
	// Verify the attestation file content
	attestation := readNamespaceAttestation(t, expectedPath)
	
	// Check that the expiration matches the expected value
	expectedExp := int64(1641003600)
	if attestation.Payload.Exp != expectedExp {
		t.Errorf("Expected Exp to be %d, got %d",
			expectedExp, attestation.Payload.Exp)
	}
}

func TestNaCreate_InvalidInput(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()
	
	// Change to temp directory for test
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)
	
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	
	// Test missing required arguments
	_, stderr, err := runLapctl(t, "na-create")
	
	// Should fail with missing arguments
	if err == nil {
		t.Error("Expected na-create to fail with missing arguments")
	}
	
	// Should contain usage information
	if stderr == "" {
		t.Error("Expected error output to contain usage information")
	}
	
	// Test missing namespace
	_, _, err = runLapctl(t, "na-create")
	if err == nil {
		t.Error("Expected na-create to fail with missing namespace")
	}
	
	// Verify no files were created
	if _, err := os.Stat("_la_namespace.json"); err == nil {
		t.Error("Expected no _la_namespace.json to be created for invalid input")
	}
}

func TestNaCreate_KeyRotation(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()
	
	// Change to temp directory for test
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)
	
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	
	// Create first attestation
	_, stderr, err := runLapctl(t, "na-create",
		"-namespace", "https://example.com/people/david/",
		"-keys-dir", "keys")
	
	if err != nil {
		t.Fatalf("First na-create failed: %v\nstderr: %s", err, stderr)
	}
	
	// Read first attestation
	attestation1 := readNamespaceAttestation(t, "_la_namespace.json")
	firstKey := attestation1.Key
	
	// Create second attestation with rotate flag
	_, stderr, err = runLapctl(t, "na-create",
		"-namespace", "https://example.com/people/david/",
		"-keys-dir", "keys",
		"-rotate")
	
	if err != nil {
		t.Fatalf("Second na-create failed: %v\nstderr: %s", err, stderr)
	}
	
	// Read second attestation
	attestation2 := readNamespaceAttestation(t, "_la_namespace.json")
	secondKey := attestation2.Key
	
	// Keys should be different due to rotation
	if firstKey == secondKey {
		t.Error("Expected different keys after rotation")
	}
	
	// Both should be valid
	if firstKey == "" || secondKey == "" {
		t.Error("Expected both keys to be valid")
	}
}

func TestRaCreate_DefaultBehavior(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()
	
	// Change to temp directory for test
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)
	
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	
	// Create a test HTML file
	testHTML := `<article><h1>Test Post</h1><p>Test content</p></article>`
	if err := os.WriteFile("test.html", []byte(testHTML), 0644); err != nil {
		t.Fatalf("Failed to create test HTML file: %v", err)
	}
	
	// Run ra-create with minimal arguments
	output, stderr, err := runLapctl(t, "ra-create",
		"-in", "test.html",
		"-url", "https://example.com/people/alice/posts/1",
		"-publisher-claim", "ac20898edf97b5a24c59749ec26ea7bc95cc1d2859ef6a194ceb7eeb2c709677",
		"-namespace-attestation-url", "https://example.com/people/alice/_la_namespace.json")
	
	if err != nil {
		t.Fatalf("ra-create failed: %v\nstderr: %s", err, stderr)
	}
	
	// Verify output contains expected messages
	if output == "" {
		t.Error("Expected output, got empty string")
	}
	
	// Check that the default file was created
	expectedPath := "_la_resource.json"
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected file %s was not created", expectedPath)
	}
	
	// Verify the resource attestation file content
	attestation := readResourceAttestation(t, expectedPath)
	
	// Check required fields
	if attestation.FragmentURL != "https://example.com/people/alice/posts/1" {
		t.Errorf("Expected FragmentURL %s, got %s",
			"https://example.com/people/alice/posts/1",
			attestation.FragmentURL)
	}
	
	if !strings.HasPrefix(attestation.Hash, "sha256:") {
		t.Errorf("Expected hash to start with 'sha256:', got %s", attestation.Hash)
	}
	
	if attestation.PublisherClaim != "ac20898edf97b5a24c59749ec26ea7bc95cc1d2859ef6a194ceb7eeb2c709677" {
		t.Errorf("Expected publisher_claim %s, got %s",
			"ac20898edf97b5a24c59749ec26ea7bc95cc1d2859ef6a194ceb7eeb2c709677",
			attestation.PublisherClaim)
	}
	
	if attestation.NamespaceAttestationURL != "https://example.com/people/alice/_la_namespace.json" {
		t.Errorf("Expected namespace_attestation_url %s, got %s",
			"https://example.com/people/alice/_la_namespace.json",
			attestation.NamespaceAttestationURL)
	}
}

func TestFragmentCreate_DefaultBehavior(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()
	
	// Change to temp directory for test
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)
	
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	
	// Create a test HTML file
	testHTML := `<article><h1>Test Post</h1><p>Test content</p></article>`
	if err := os.WriteFile("test.html", []byte(testHTML), 0644); err != nil {
		t.Fatalf("Failed to create test HTML file: %v", err)
	}
	
	// Run fragment-create with minimal arguments
	output, stderr, err := runLapctl(t, "fragment-create",
		"-in", "test.html",
		"-url", "https://example.com/people/alice/posts/1",
		"-publisher-claim", "ac20898edf97b5a24c59749ec26ea7bc95cc1d2859ef6a194ceb7eeb2c709677",
		"-resource-attestation-url", "https://example.com/people/alice/posts/1/_la_resource.json",
		"-namespace-attestation-url", "https://example.com/people/alice/_la_namespace.json")
	
	if err != nil {
		t.Fatalf("fragment-create failed: %v\nstderr: %s", err, stderr)
	}
	
	// Verify output contains expected messages
	if output == "" {
		t.Error("Expected output, got empty string")
	}
	
	// Check that the default file was created
	expectedPath := "index.htmx"
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected file %s was not created", expectedPath)
	}
	
	// Read and verify the fragment file content
	fragmentBytes, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("Failed to read fragment file: %v", err)
	}
	
	fragmentContent := string(fragmentBytes)
	
	// Check for required v0.2 structure elements
	if !strings.Contains(fragmentContent, `data-la-spec="v0.2"`) {
		t.Error("Expected fragment to contain data-la-spec=\"v0.2\"")
	}
	
	if !strings.Contains(fragmentContent, `data-la-fragment-url="https://example.com/people/alice/posts/1"`) {
		t.Error("Expected fragment to contain correct data-la-fragment-url")
	}
	
	if !strings.Contains(fragmentContent, `<section class="la-preview">`) {
		t.Error("Expected fragment to contain la-preview section")
	}
	
	if !strings.Contains(fragmentContent, `data-la-publisher-claim="ac20898edf97b5a24c59749ec26ea7bc95cc1d2859ef6a194ceb7eeb2c709677"`) {
		t.Error("Expected fragment to contain correct data-la-publisher-claim")
	}
	
	if !strings.Contains(fragmentContent, `data-la-resource-attestation-url="https://example.com/people/alice/posts/1/_la_resource.json"`) {
		t.Error("Expected fragment to contain correct data-la-resource-attestation-url")
	}
	
	if !strings.Contains(fragmentContent, `data-la-namespace-attestation-url="https://example.com/people/alice/_la_namespace.json"`) {
		t.Error("Expected fragment to contain correct data-la-namespace-attestation-url")
	}
	
	if !strings.Contains(fragmentContent, `href="data:text/html;base64,`) {
		t.Error("Expected fragment to contain base64-encoded content in href")
	}
}

func TestUpdatePosts_CompleteWorkflow(t *testing.T) {
	tmpDir, cleanup := setupTestEnvironment(t)
	defer cleanup()
	
	// Change to temp directory for test
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(originalDir)
	
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}
	
	// Create the required directory structure
	if err := os.MkdirAll("posts/1", 0755); err != nil {
		t.Fatalf("Failed to create posts/1 directory: %v", err)
	}
	if err := os.MkdirAll("posts/2", 0755); err != nil {
		t.Fatalf("Failed to create posts/2 directory: %v", err)
	}
	if err := os.MkdirAll("posts/3", 0755); err != nil {
		t.Fatalf("Failed to create posts/3 directory: %v", err)
	}
	
	// Create test HTML files for each post
	post1HTML := `<article><h1>Post 1</h1><p>First post content</p></article>`
	if err := os.WriteFile("posts/1/index.html", []byte(post1HTML), 0644); err != nil {
		t.Fatalf("Failed to create post 1 HTML: %v", err)
	}
	
	post2HTML := `<article><h1>Post 2</h1><p>Second post content</p></article>`
	if err := os.WriteFile("posts/2/index.html", []byte(post2HTML), 0644); err != nil {
		t.Fatalf("Failed to create post 2 HTML: %v", err)
	}
	
	post3HTML := `<article><h1>Post 3</h1><p>Third post content</p></article>`
	if err := os.WriteFile("posts/3/index.html", []byte(post3HTML), 0644); err != nil {
		t.Fatalf("Failed to create post 3 HTML: %v", err)
	}
	
	// Create host HTML file
	hostHTML := `<!DOCTYPE html>
<html>
<head><title>Test Posts</title></head>
<body>
<article data-la-fragment-url="https://example.com/people/alice/posts/1">
<p>Loading post 1...</p>
</article>
<article data-la-fragment-url="https://example.com/people/alice/posts/2">
<p>Loading post 2...</p>
</article>
<article data-la-fragment-url="https://example.com/people/alice/posts/3">
<p>Loading post 3...</p>
</article>
</body>
</html>`
	if err := os.WriteFile("posts/index.html", []byte(hostHTML), 0644); err != nil {
		t.Fatalf("Failed to create host HTML: %v", err)
	}
	
	// Create a test publisher key
	keysDir := "keys"
	if err := os.MkdirAll(keysDir, 0700); err != nil {
		t.Fatalf("Failed to create keys directory: %v", err)
	}
	
	testKey := `{
		"privkey_hex": "b390add8da13892d0a4ca22ef5aa5f8efd4c0331bd3c2b3ce28eade7beac0c5b",
		"pubkey_xonly_hex": "ac20898edf97b5a24c59749ec26ea7bc95cc1d2859ef6a194ceb7eeb2c709677",
		"created_at": 1756388071
	}`
	if err := os.WriteFile(filepath.Join(keysDir, "alice_publisher_key.json"), []byte(testKey), 0600); err != nil {
		t.Fatalf("Failed to create test key: %v", err)
	}
	
	// Run update-posts
	output, stderr, err := runLapctl(t, "update-posts",
		"-base", "https://example.com",
		"-dir", ".")
	
	if err != nil {
		t.Fatalf("update-posts failed: %v\nstderr: %s", err, stderr)
	}
	
	// Verify output contains expected messages
	if output == "" {
		t.Error("Expected output, got empty string")
	}
	
	// Check that all expected files were created
	expectedFiles := []string{
		"posts/1/_la_resource.json",
		"posts/1/index.htmx",
		"posts/2/_la_resource.json",
		"posts/2/index.htmx",
		"posts/3/_la_resource.json",
		"posts/3/index.htmx",
	}
	
	for _, file := range expectedFiles {
		if _, err := os.Stat(file); os.IsNotExist(err) {
			t.Errorf("Expected file %s was not created", file)
		}
	}
	
	// Check that the host file was updated
	if _, err := os.Stat("posts/index.html.bak"); os.IsNotExist(err) {
		t.Error("Expected backup file posts/index.html.bak was not created")
	}
	
	// Verify the updated host file contains the fragments
	updatedHostBytes, err := os.ReadFile("posts/index.html")
	if err != nil {
		t.Fatalf("Failed to read updated host file: %v", err)
	}
	
	updatedHostContent := string(updatedHostBytes)
	
	// Check that the fragments were embedded
	if !strings.Contains(updatedHostContent, `data-la-spec="v0.2"`) {
		t.Error("Expected updated host to contain v0.2 fragments")
	}
	
	if !strings.Contains(updatedHostContent, `<section class="la-preview">`) {
		t.Error("Expected updated host to contain la-preview sections")
	}
	
	// Check that all three posts are present
	if !strings.Contains(updatedHostContent, "First post content") {
		t.Error("Expected post 1 content to be embedded")
	}
	if !strings.Contains(updatedHostContent, "Second post content") {
		t.Error("Expected post 2 content to be embedded")
	}
	if !strings.Contains(updatedHostContent, "Third post content") {
		t.Error("Expected post 3 content to be embedded")
	}
}
