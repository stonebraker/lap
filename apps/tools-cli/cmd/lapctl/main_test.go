package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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
		"-namespace", "https://example.com/people/alice/",
		"-kid", "test-key-default")
	
	if err != nil {
		t.Fatalf("na-create failed: %v\nstderr: %s", err, stderr)
	}
	
	// Verify output contains expected messages
	if output == "" {
		t.Error("Expected output, got empty string")
	}
	
	// Check that the default file was created (using the default path from the command)
	expectedPath := "la_namespace.json"
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected file %s was not created", expectedPath)
	}
	
	// Verify the attestation file content
	attestation := readNamespaceAttestation(t, expectedPath)
	
	// Check required fields
	if len(attestation.Payload.Namespace) == 0 {
		t.Error("Expected namespace array to be populated")
	}
	
	if attestation.Payload.Namespace[0] != "https://example.com/people/alice/" {
		t.Errorf("Expected namespace %s, got %s", 
			"https://example.com/people/alice/", 
			attestation.Payload.Namespace[0])
	}
	
	if attestation.Payload.AttestationPath != "la_namespace.json" {
		t.Errorf("Expected attestation_path %s, got %s",
			"la_namespace.json",
			attestation.Payload.AttestationPath)
	}
	
	if attestation.Payload.KID != "test-key-default" {
		t.Errorf("Expected KID %s, got %s", "test-key-default", attestation.Payload.KID)
	}
	
	// Check timestamps
	now := time.Now().Unix()
	if attestation.Payload.IAT == 0 {
		t.Error("Expected IAT to be set")
	}
	if attestation.Payload.EXP == 0 {
		t.Error("Expected EXP to be set")
	}
	if attestation.Payload.EXP <= attestation.Payload.IAT {
		t.Error("Expected EXP to be after IAT")
	}
	// Verify timestamps are reasonable (not in the distant past/future)
	if attestation.Payload.IAT < now-3600 || attestation.Payload.IAT > now+3600 {
		t.Errorf("Expected IAT to be within 1 hour of now, got %d (now: %d)", attestation.Payload.IAT, now)
	}
	
	// Check signature fields
	if attestation.PublisherKey == "" {
		t.Error("Expected publisher_key to be set")
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
		"-kid", "test-key-custom",
		"-out", "custom-output",
		"-path", "my-attestation.json")
	
	if err != nil {
		t.Fatalf("na-create failed: %v\nstderr: %s", err, stderr)
	}
	
	// Verify output contains expected messages
	if output == "" {
		t.Error("Expected output, got empty string")
	}
	
	// Check that the custom file was created
	expectedPath := filepath.Join("custom-output", "my-attestation.json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected file %s was not created", expectedPath)
	}
	
	// Verify the attestation file content
	attestation := readNamespaceAttestation(t, expectedPath)
	
	// Check that attestation_path field matches the custom path
	if attestation.Payload.AttestationPath != "my-attestation.json" {
		t.Errorf("Expected attestation_path %s, got %s",
			"my-attestation.json",
			attestation.Payload.AttestationPath)
	}
	
	// Check namespace
	if attestation.Payload.Namespace[0] != "https://example.com/people/bob/" {
		t.Errorf("Expected namespace %s, got %s",
			"https://example.com/people/bob/",
			attestation.Payload.Namespace[0])
	}
}

func TestNaCreate_CustomWindow(t *testing.T) {
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
	
	// Run na-create with custom window
	output, stderr, err := runLapctl(t, "na-create",
		"-namespace", "https://example.com/people/charlie/",
		"-kid", "test-key-window",
		"-window", "1h")
	
	if err != nil {
		t.Fatalf("na-create failed: %v\nstderr: %s", err, stderr)
	}
	
	// Verify output contains expected messages
	if output == "" {
		t.Error("Expected output, got empty string")
	}
	
	// Check that the file was created (using the default path)
	expectedPath := "la_namespace.json"
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("Expected file %s was not created", expectedPath)
	}
	
	// Verify the attestation file content
	attestation := readNamespaceAttestation(t, expectedPath)
	
	// Check that the window is approximately 1 hour
	expectedExp := time.Now().Unix() + 3600 // 1 hour in seconds
	
	// Allow some tolerance for execution time
	tolerance := int64(10) // 10 seconds
	if attestation.Payload.EXP < expectedExp-tolerance || 
	   attestation.Payload.EXP > expectedExp+tolerance {
		t.Errorf("Expected EXP to be approximately %d (1 hour from now), got %d",
			expectedExp, attestation.Payload.EXP)
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
	_, stderr, err = runLapctl(t, "na-create", "-kid", "test-key")
	if err == nil {
		t.Error("Expected na-create to fail with missing namespace")
	}
	
	// Test missing kid
	_, stderr, err = runLapctl(t, "na-create", "-namespace", "https://example.com/")
	if err == nil {
		t.Error("Expected na-create to fail with missing kid")
	}
	
	// Verify no files were created
	if _, err := os.Stat("_lap"); err == nil {
		t.Error("Expected no _lap directory to be created for invalid input")
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
		"-kid", "test-key-rotation",
		"-keys-dir", "keys")
	
	if err != nil {
		t.Fatalf("First na-create failed: %v\nstderr: %s", err, stderr)
	}
	
	// Read first attestation
	attestation1 := readNamespaceAttestation(t, "la_namespace.json")
	firstKey := attestation1.PublisherKey
	
	// Create second attestation with rotate flag
	_, stderr, err = runLapctl(t, "na-create",
		"-namespace", "https://example.com/people/david/",
		"-kid", "test-key-rotation",
		"-keys-dir", "keys",
		"-rotate")
	
	if err != nil {
		t.Fatalf("Second na-create failed: %v\nstderr: %s", err, stderr)
	}
	
	// Read second attestation
	attestation2 := readNamespaceAttestation(t, "la_namespace.json")
	secondKey := attestation2.PublisherKey
	
	// Keys should be different due to rotation
	if firstKey == secondKey {
		t.Error("Expected different keys after rotation")
	}
	
	// Both should be valid
	if firstKey == "" || secondKey == "" {
		t.Error("Expected both keys to be valid")
	}
}
