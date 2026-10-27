package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/daFish/gogo-meta/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecRefusesAConfigTheSchemaRejects(t *testing.T) {
	const validPrimary = `{"projects":{"api":"git@example.com:org/api.git"}}`
	cases := []struct {
		name     string
		files    map[string]string
		args     []string
		rejected string
	}{
		{
			name:     "accepted config runs the command",
			files:    map[string]string{".gogo": validPrimary},
			args:     []string{"exec", "touch marker"},
			rejected: "",
		},
		{
			name: "unknown key in the primary config",
			files: map[string]string{".gogo": `{"projects":{"api":"git@example.com:org/api.git"},` +
				`"commands":{"test":{"cmd":"npm test","parallell":true}}}`},
			args:     []string{"exec", "touch marker"},
			rejected: `.gogo: commands.test: unknown key "parallell"`,
		},
		{
			name: "several problems in one file",
			files: map[string]string{".gogo": `{"projects":{"api":"git@example.com:org/api.git"},` +
				`"group":{},"commands":{"b":{"cmd":"x","concurrency":0}}}`},
			args:     []string{"exec", "touch marker"},
			rejected: `.gogo: unknown key "group"`,
		},
		{
			name:     "unknown key in .gogo.local",
			files:    map[string]string{".gogo": validPrimary, ".gogo.local": `{"group":{}}`},
			args:     []string{"exec", "touch marker"},
			rejected: `.gogo.local: unknown key "group"`,
		},
		{
			name:     "unknown key in an overlay",
			files:    map[string]string{".gogo": validPrimary, ".gogo.devops.yaml": "ignores: []\n"},
			args:     []string{"-f", ".gogo.devops.yaml", "exec", "touch marker"},
			rejected: `.gogo.devops.yaml: unknown key "ignores"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, content := range tc.files {
				require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
			}
			require.NoError(t, os.MkdirAll(filepath.Join(dir, "api"), 0o755))
			out := captureOutput(t)
			initTestChdir(t, dir)
			t.Cleanup(func() { config.SetOverlayFiles(nil) })

			root := NewRootCommand("test")
			root.SetArgs(tc.args)
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			err := root.Execute()

			marker := filepath.Join(dir, "api", "marker")
			if tc.rejected == "" {
				require.NoError(t, err)
				assert.FileExists(t, marker)
				return
			}
			require.Error(t, err)
			assert.True(t, strings.HasPrefix(err.Error(), tc.rejected),
				"the file is named relative to the working directory: %q", err.Error())
			file := strings.SplitN(tc.rejected, ": ", 2)[0]
			for line := range strings.SplitSeq(err.Error(), "\n") {
				assert.True(t, strings.HasPrefix(line, file+": "), "every problem names its file: %q", line)
			}
			assert.NoFileExists(t, marker, "nothing may run with a config the schema rejects")
			assert.NotContains(t, out.String(), "Executing", "the command is announced only once the config has loaded")
		})
	}
}
