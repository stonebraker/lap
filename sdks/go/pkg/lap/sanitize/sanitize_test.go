package sanitize

import (
	"testing"
)

func TestSanitizeCanonicalContent(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "basic HTML",
			input:    []byte("<p>Hello world</p>"),
			expected: "<p>Hello world</p>",
		},
		{
			name:     "script tag removal",
			input:    []byte("<p>Hello</p><script>alert('xss')</script><p>World</p>"),
			expected: "<p>Hello</p><p>World</p>",
		},
		{
			name:     "event handler removal",
			input:    []byte("<p onclick=\"alert('xss')\">Click me</p>"),
			expected: "<p>Click me</p>",
		},
		{
			name:     "javascript URL removal",
			input:    []byte("<a href=\"javascript:alert('xss')\">Link</a>"),
			expected: "Link",
		},
		{
			name:     "iframe removal",
			input:    []byte("<p>Content</p><iframe src=\"evil.com\"></iframe>"),
			expected: "<p>Content</p>",
		},
		{
			name:     "object/embed removal",
			input:    []byte("<p>Content</p><object data=\"evil.swf\"></object>"),
			expected: "<p>Content</p>",
		},
		{
			name:     "form removal",
			input:    []byte("<p>Content</p><form action=\"evil.com\"><input type=\"text\"></form>"),
			expected: "<p>Content</p>",
		},
		{
			name:     "style attribute removal",
			input:    []byte("<p style=\"color: red; background: url('javascript:alert(1)')\">Styled</p>"),
			expected: "<p>Styled</p>",
		},
		{
			name:     "complex XSS attempt",
			input:    []byte("<img src=\"x\" onerror=\"alert('xss')\" /><script>document.location='evil.com'</script>"),
			expected: "<img src=\"x\"/>",
		},
		{
			name:     "empty input",
			input:    []byte(""),
			expected: "",
		},
		{
			name:     "plain text",
			input:    []byte("Just plain text"),
			expected: "Just plain text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SanitizeCanonicalContent(tt.input)
			if err != nil {
				t.Fatalf("SanitizeCanonicalContent() error = %v", err)
			}
			if result != tt.expected {
				t.Errorf("SanitizeCanonicalContent() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSanitizeCanonicalContent_RealLAPFragment(t *testing.T) {
	// Test with a real LAP fragment structure
	canonicalContent := []byte(`<article>
    <header>
        <h2>Post 3</h2>
        <p>2 days ago • 11:37 AM</p>
    </header>
    <p>
        Made some Pakhlava for dessert tonight Layers of flaky pastry, nuts, and
        honey... yum!
    </p>
</article>`)

	result, err := SanitizeCanonicalContent(canonicalContent)
	if err != nil {
		t.Fatalf("SanitizeCanonicalContent() error = %v", err)
	}

	// Should preserve the structure but remove any dangerous elements
	if result == "" {
		t.Error("Expected non-empty result for valid HTML")
	}

	// Should not contain script tags
	if contains(result, "<script") {
		t.Error("Result should not contain script tags")
	}

	// Should not contain event handlers
	if contains(result, "onclick") || contains(result, "onerror") || contains(result, "onload") {
		t.Error("Result should not contain event handlers")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || 
		s[len(s)-len(substr):] == substr || 
		containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
