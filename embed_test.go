package commandcenter

import (
	"io/fs"
	"testing"
)

func TestWebFSContainsBuildOutput(t *testing.T) {
	// Verify the embed.FS is accessible and contains the expected structure
	_, err := fs.Stat(WebFS, "web/build/index.html")
	if err != nil {
		t.Fatalf("expected web/build/index.html in embedded FS, got error: %v", err)
	}
}

func TestWebFSSubDirectoryAccessible(t *testing.T) {
	sub, err := fs.Sub(WebFS, "web/build")
	if err != nil {
		t.Fatalf("failed to create sub filesystem: %v", err)
	}

	_, err = fs.Stat(sub, "index.html")
	if err != nil {
		t.Fatalf("expected index.html in sub filesystem, got error: %v", err)
	}
}
