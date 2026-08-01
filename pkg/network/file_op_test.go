package network

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal.txt", "normal.txt"},
		{"file:name?.txt", "file name .txt"},
		{"COM1", "miru_COM1"},
		{"CON.txt", "miru_CON.txt"},
		{"   ", "unnamed_file"},
		{"a/b\\c", "a b c"},
	}

	for _, test := range tests {
		result := SanitizeFilename(test.input)
		if result != test.expected {
			t.Errorf("SanitizeFilename(%q) = %q; want %q", test.input, result, test.expected)
		}
	}
}

func TestSanitizeFolderPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"a/b:c/d?", filepath.Join("a", "b c", "d")},
		{"C:\\unsafe:dir?\\sub*dir", filepath.Join("C", "unsafe dir", "sub dir")},
		{"", "."},
	}

	for _, test := range tests {
		result := SanitizeFolderPath(test.input)
		if result != test.expected {
			t.Errorf("SanitizeFolderPath(%q) = %q; want %q", test.input, result, test.expected)
		}
	}
}

func TestTouchFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-touchfile-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Test TouchFile with unsafe path components
	unsafePath := filepath.Join(tempDir, "unsafe:dir?", "file*name.txt")
	file, err := TouchFile(unsafePath)
	if err != nil {
		t.Fatalf("TouchFile failed: %v", err)
	}
	file.Close()

	// Verify that the sanitized directory was created and file was created
	expectedDir := filepath.Join(tempDir, "unsafe dir", "file name.txt")
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Errorf("expected file to exist at %s, but got error: %v", expectedDir, err)
	}
}
