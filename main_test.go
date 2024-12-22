package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitInteractions(t *testing.T) {
	remote := createTestRepo(t)
	require.NoError(t, os.Chdir(t.TempDir()))

	require.NoError(t, initializeRepo(remote))
	require.NoError(t, initializeRepo(remote))

	// Read a page
	content, found, err := readPage("foo/test")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "<h1 id=\"hello\">hello</h1>\n\n<p><strong>world</strong></p>\n", content)

	// Read a page that doesn't exist
	content, found, err = readPage("foo/bar")
	require.NoError(t, err)
	assert.False(t, found)
	assert.Empty(t, content)

	// Update a page
	err = stageUpdate("foo/test", "<h1 id=\"hello\">hello again</h1>\n\n<p><strong>world</strong></p>\n", "user@test.com")
	require.NoError(t, err)

	// Confirm update
	content, found, err = readPage("foo/test")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Equal(t, "<h1 id=\"hello-again\">hello again</h1>\n\n<p><strong>world</strong></p>\n", content)

	// No-op update
	err = stageUpdate("foo/test", "<h1 id=\"hello\">hello again</h1>\n\n<p><strong>world</strong></p>\n", "user@test.com")
	require.NoError(t, err)

	// Update a page that doesn't exist
	err = stageUpdate("foo/bar", "<h1 id=\"hello\">hello again</h1>\n\n<p><strong>world</strong></p>\n", "user@test.com")
	require.Error(t, err)

	// Update the remote
	require.NoError(t, pushPull())
	require.NoError(t, pushPull())

	// Confirm the remote was updated
	require.NoError(t, os.Chdir(t.TempDir()))
	require.NoError(t, git("clone", remote, "."))
	raw, err := os.ReadFile(filepath.Join("content", "foo", "test.md"))
	require.NoError(t, err)
	assert.Equal(t, "# hello again\n\n**world**", string(raw))
}

func createTestRepo(t *testing.T) string {
	dir := t.TempDir()
	require.NoError(t, os.Chdir(dir))
	require.NoError(t, git("init", "--bare"))

	require.NoError(t, os.Chdir(t.TempDir()))
	require.NoError(t, git("clone", dir, "."))
	require.NoError(t, os.MkdirAll(filepath.Join("content", "foo"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join("content", "foo", "test.md"), []byte("# hello\n__world__\n"), 0755))
	require.NoError(t, git("add", "."))
	require.NoError(t, git("commit", "-m", "initial commit"))
	require.NoError(t, git("push", "origin", "main"))

	return dir
}
