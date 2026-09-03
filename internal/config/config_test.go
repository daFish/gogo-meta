package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

func TestCreateDefaultConfig(t *testing.T) {
	config := CreateDefaultConfig()
	assert.Equal(t, map[string]string{}, config.Projects)
	assert.Contains(t, config.Ignore, ".git")
	assert.Contains(t, config.Ignore, "node_modules")
}

func TestAddProject(t *testing.T) {
	t.Run("adds a new project", func(t *testing.T) {
		config := CreateDefaultConfig()
		updated := AddProject(config, "libs/core", "git@github.com:org/core.git")
		assert.Equal(t, "git@github.com:org/core.git", updated.Projects["libs/core"])
	})

	t.Run("preserves existing projects", func(t *testing.T) {
		config := MetaConfig{Projects: map[string]string{"existing": "url1"}, Ignore: []string{}}
		updated := AddProject(config, "new", "url2")
		assert.Equal(t, "url1", updated.Projects["existing"])
		assert.Equal(t, "url2", updated.Projects["new"])
	})

	t.Run("does not mutate original", func(t *testing.T) {
		config := CreateDefaultConfig()
		AddProject(config, "test", "url")
		assert.Equal(t, map[string]string{}, config.Projects)
	})
}

func TestRemoveProject(t *testing.T) {
	t.Run("removes a project", func(t *testing.T) {
		config := MetaConfig{Projects: map[string]string{"a": "url1", "b": "url2"}, Ignore: []string{}}
		updated := RemoveProject(config, "a")
		_, ok := updated.Projects["a"]
		assert.False(t, ok)
		assert.Equal(t, "url2", updated.Projects["b"])
	})

	t.Run("handles removing non-existent project", func(t *testing.T) {
		config := CreateDefaultConfig()
		updated := RemoveProject(config, "nonexistent")
		assert.Empty(t, updated.Projects)
	})
}

func TestGetProjectPaths(t *testing.T) {
	t.Run("returns all paths sorted", func(t *testing.T) {
		config := MetaConfig{Projects: map[string]string{"libs/b": "url2", "libs/a": "url1"}, Ignore: []string{}}
		assert.Equal(t, []string{"libs/a", "libs/b"}, GetProjectPaths(config))
	})

	t.Run("returns empty for no projects", func(t *testing.T) {
		config := CreateDefaultConfig()
		assert.Empty(t, GetProjectPaths(config))
	})
}

func TestGetProjectURL(t *testing.T) {
	t.Run("returns url for existing project", func(t *testing.T) {
		config := MetaConfig{Projects: map[string]string{"test": "git@example.com:test.git"}, Ignore: []string{}}
		url, ok := GetProjectURL(config, "test")
		assert.True(t, ok)
		assert.Equal(t, "git@example.com:test.git", url)
	})

	t.Run("returns false for non-existent", func(t *testing.T) {
		config := CreateDefaultConfig()
		_, ok := GetProjectURL(config, "nonexistent")
		assert.False(t, ok)
	})
}

func TestDetectFormat(t *testing.T) {
	tests := []struct {
		path   string
		expect ConfigFormat
	}{
		{"/project/.gogo", FormatJSON},
		{"/project/.gogo.yaml", FormatYAML},
		{"/project/.gogo.yml", FormatYAML},
		{"/project/.gogo.toml", FormatJSON},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expect, DetectFormat(tt.path))
		})
	}
}

func TestFilenameForFormat(t *testing.T) {
	assert.Equal(t, ".gogo", FilenameForFormat(FormatJSON))
	assert.Equal(t, ".gogo.yaml", FilenameForFormat(FormatYAML))
}

func TestFindFileUp(t *testing.T) {
	t.Run("finds file in current directory", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"), `{}`)

		result, err := FindFileUp(".gogo", dir)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, ".gogo"), result)
	})

	t.Run("finds file in parent directory", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"), `{}`)
		sub := filepath.Join(dir, "sub")
		require.NoError(t, os.MkdirAll(sub, 0o755))

		result, err := FindFileUp(".gogo", sub)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, ".gogo"), result)
	})

	t.Run("returns empty when not found", func(t *testing.T) {
		dir := t.TempDir()
		result, err := FindFileUp(".gogo", dir)
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestFindMetaFileUp(t *testing.T) {
	t.Run("finds .gogo", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"), `{"projects":{}}`)

		result, err := FindMetaFileUp(dir)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, ".gogo"), result)
	})

	t.Run("finds .gogo.yaml", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo.yaml"), "projects: {}")

		result, err := FindMetaFileUp(dir)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, ".gogo.yaml"), result)
	})

	t.Run("finds .gogo.yml", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo.yml"), "projects: {}")

		result, err := FindMetaFileUp(dir)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, ".gogo.yml"), result)
	})

	t.Run("prefers .gogo over .gogo.yaml", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"), `{"projects":{}}`)
		writeFile(t, filepath.Join(dir, ".gogo.yaml"), "projects: {}")

		result, err := FindMetaFileUp(dir)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, ".gogo"), result)
	})

	t.Run("prefers .gogo.yaml over .gogo.yml", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo.yaml"), "projects: {}")
		writeFile(t, filepath.Join(dir, ".gogo.yml"), "projects: {}")

		result, err := FindMetaFileUp(dir)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, ".gogo.yaml"), result)
	})

	t.Run("finds config in parent directory", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo.yaml"), "projects: {}")
		sub := filepath.Join(dir, "sub")
		require.NoError(t, os.MkdirAll(sub, 0o755))

		result, err := FindMetaFileUp(sub)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(dir, ".gogo.yaml"), result)
	})

	t.Run("prefers child directory match over parent", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"), `{"projects":{}}`)
		child := filepath.Join(dir, "child")
		writeFile(t, filepath.Join(child, ".gogo.yaml"), "projects: {}")

		result, err := FindMetaFileUp(child)
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(child, ".gogo.yaml"), result)
	})

	t.Run("returns empty when not found", func(t *testing.T) {
		dir := t.TempDir()
		result, err := FindMetaFileUp(dir)
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestReadMetaConfig(t *testing.T) {
	t.Run("reads valid .gogo file", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"), `{"projects":{"libs/core":"git@github.com:org/core.git"},"ignore":[".git"]}`)

		result, err := ReadMetaConfig(dir, nil)
		require.NoError(t, err)
		assert.Equal(t, "git@github.com:org/core.git", result.Config.Projects["libs/core"])
		assert.Contains(t, result.Config.Ignore, ".git")
		assert.Equal(t, FormatJSON, result.Format)
		assert.Equal(t, dir, result.MetaDir)
	})

	t.Run("throws ConfigError when not found", func(t *testing.T) {
		dir := t.TempDir()
		_, err := ReadMetaConfig(dir, nil)
		assert.Error(t, err)
		var configErr *ConfigError
		assert.ErrorAs(t, err, &configErr)
	})

	t.Run("throws ConfigError for invalid JSON", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"), "not json")

		_, err := ReadMetaConfig(dir, nil)
		assert.Error(t, err)
		var configErr *ConfigError
		assert.ErrorAs(t, err, &configErr)
	})

	t.Run("provides default ignore when not specified", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"), `{"projects":{}}`)

		result, err := ReadMetaConfig(dir, nil)
		require.NoError(t, err)
		assert.Contains(t, result.Config.Ignore, ".git")
		assert.Contains(t, result.Config.Ignore, "node_modules")
	})
}

func TestReadMetaConfigYAML(t *testing.T) {
	t.Run("reads valid .gogo.yaml", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo.yaml"), "projects:\n  libs/core: \"git@github.com:org/core.git\"\nignore:\n  - .git\n")

		result, err := ReadMetaConfig(dir, nil)
		require.NoError(t, err)
		assert.Equal(t, FormatYAML, result.Format)
		assert.Equal(t, "git@github.com:org/core.git", result.Config.Projects["libs/core"])
	})

	t.Run("reads valid .gogo.yml", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo.yml"), "projects:\n  api: \"git@github.com:org/api.git\"\nignore:\n  - .git\n")

		result, err := ReadMetaConfig(dir, nil)
		require.NoError(t, err)
		assert.Equal(t, FormatYAML, result.Format)
		assert.Equal(t, "git@github.com:org/api.git", result.Config.Projects["api"])
	})

	t.Run("throws ConfigError for invalid YAML", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo.yaml"), "{ invalid yaml: [")

		_, err := ReadMetaConfig(dir, nil)
		assert.Error(t, err)
		var configErr *ConfigError
		assert.ErrorAs(t, err, &configErr)
	})

	t.Run("provides default ignore for YAML", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo.yaml"), "projects: {}")

		result, err := ReadMetaConfig(dir, nil)
		require.NoError(t, err)
		assert.Contains(t, result.Config.Ignore, ".git")
		assert.Contains(t, result.Config.Ignore, "node_modules")
	})

	t.Run("parses YAML with commands", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo.yaml"), "projects: {}\ncommands:\n  build: npm run build\n  test:\n    cmd: npm test\n    parallel: true\n")

		result, err := ReadMetaConfig(dir, nil)
		require.NoError(t, err)
		assert.Equal(t, "npm run build", result.Config.Commands["build"].Cmd)
		assert.Equal(t, "npm test", result.Config.Commands["test"].Cmd)
		assert.NotNil(t, result.Config.Commands["test"].Parallel)
		assert.True(t, *result.Config.Commands["test"].Parallel)
	})
}

func TestWriteMetaConfig(t *testing.T) {
	t.Run("writes JSON config", func(t *testing.T) {
		dir := t.TempDir()
		config := MetaConfig{Projects: map[string]string{"test": "url"}, Ignore: []string{".git"}}

		err := WriteMetaConfig(dir, config, FormatJSON)
		require.NoError(t, err)

		content, err := os.ReadFile(filepath.Join(dir, ".gogo"))
		require.NoError(t, err)

		var parsed map[string]any
		require.NoError(t, json.Unmarshal(content, &parsed))
		projects := parsed["projects"].(map[string]any)
		assert.Equal(t, "url", projects["test"])
	})

	t.Run("writes YAML config", func(t *testing.T) {
		dir := t.TempDir()
		config := MetaConfig{Projects: map[string]string{"test": "url"}, Ignore: []string{".git"}}

		err := WriteMetaConfig(dir, config, FormatYAML)
		require.NoError(t, err)

		assert.FileExists(t, filepath.Join(dir, ".gogo.yaml"))
		assert.NoFileExists(t, filepath.Join(dir, ".gogo"))
		content, err := os.ReadFile(filepath.Join(dir, ".gogo.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(content), "projects:")
		assert.Contains(t, string(content), "test: url")
	})

	t.Run("defaults to JSON format", func(t *testing.T) {
		dir := t.TempDir()
		err := WriteMetaConfig(dir, CreateDefaultConfig(), FormatJSON)
		require.NoError(t, err)

		assert.FileExists(t, filepath.Join(dir, ".gogo"))
		assert.NoFileExists(t, filepath.Join(dir, ".gogo.yaml"))
	})
}

func TestMergeConfigs(t *testing.T) {
	t.Run("unions projects", func(t *testing.T) {
		base := MetaConfig{Projects: map[string]string{"a": "url1"}, Ignore: []string{}}
		overlay := MetaConfig{Projects: map[string]string{"b": "url2"}, Ignore: []string{}}
		merged := MergeConfigs(base, overlay)
		assert.Equal(t, "url1", merged.Projects["a"])
		assert.Equal(t, "url2", merged.Projects["b"])
	})

	t.Run("overlay wins on conflict", func(t *testing.T) {
		base := MetaConfig{Projects: map[string]string{"a": "url1"}, Ignore: []string{}}
		overlay := MetaConfig{Projects: map[string]string{"a": "url2"}, Ignore: []string{}}
		merged := MergeConfigs(base, overlay)
		assert.Equal(t, "url2", merged.Projects["a"])
	})

	t.Run("deduplicates ignore", func(t *testing.T) {
		base := MetaConfig{Projects: map[string]string{}, Ignore: []string{".git", "node_modules"}}
		overlay := MetaConfig{Projects: map[string]string{}, Ignore: []string{"node_modules", "dist"}}
		merged := MergeConfigs(base, overlay)
		assert.Equal(t, []string{".git", "node_modules", "dist"}, merged.Ignore)
	})

	t.Run("unions commands with overlay winning", func(t *testing.T) {
		base := MetaConfig{
			Projects: map[string]string{}, Ignore: []string{},
			Commands: map[string]CommandConfig{
				"build": {Cmd: "npm run build"},
				"test":  {Cmd: "npm test"},
			},
		}
		overlay := MetaConfig{
			Projects: map[string]string{}, Ignore: []string{},
			Commands: map[string]CommandConfig{
				"test":   {Cmd: "bun test"},
				"deploy": {Cmd: "npm run deploy"},
			},
		}
		merged := MergeConfigs(base, overlay)
		assert.Equal(t, "npm run build", merged.Commands["build"].Cmd)
		assert.Equal(t, "bun test", merged.Commands["test"].Cmd)
		assert.Equal(t, "npm run deploy", merged.Commands["deploy"].Cmd)
	})

	t.Run("handles nil commands in base", func(t *testing.T) {
		base := MetaConfig{Projects: map[string]string{}, Ignore: []string{}}
		overlay := MetaConfig{Projects: map[string]string{}, Ignore: []string{}, Commands: map[string]CommandConfig{"build": {Cmd: "npm run build"}}}
		merged := MergeConfigs(base, overlay)
		assert.Equal(t, "npm run build", merged.Commands["build"].Cmd)
	})

	t.Run("handles nil commands in overlay", func(t *testing.T) {
		base := MetaConfig{Projects: map[string]string{}, Ignore: []string{}, Commands: map[string]CommandConfig{"build": {Cmd: "npm run build"}}}
		overlay := MetaConfig{Projects: map[string]string{}, Ignore: []string{}}
		merged := MergeConfigs(base, overlay)
		assert.Equal(t, "npm run build", merged.Commands["build"].Cmd)
	})

	t.Run("handles nil commands in both", func(t *testing.T) {
		base := MetaConfig{Projects: map[string]string{}, Ignore: []string{}}
		overlay := MetaConfig{Projects: map[string]string{}, Ignore: []string{}}
		merged := MergeConfigs(base, overlay)
		assert.Nil(t, merged.Commands)
	})
}

func TestReadMetaConfigWithOverlays(t *testing.T) {
	t.Run("returns primary only when overlays empty", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"), `{"projects":{"a":"url1"}}`)
		writeFile(t, filepath.Join(dir, ".gogo.devops"), `{"projects":{"b":"url2"}}`)

		result, err := ReadMetaConfig(dir, []string{})
		require.NoError(t, err)
		assert.Equal(t, "url1", result.Config.Projects["a"])
		_, ok := result.Config.Projects["b"]
		assert.False(t, ok)
	})

	t.Run("merges a single overlay", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"), `{"projects":{"a":"url1"}}`)
		writeFile(t, filepath.Join(dir, ".gogo.devops"), `{"projects":{"b":"url2"}}`)

		result, err := ReadMetaConfig(dir, []string{".gogo.devops"})
		require.NoError(t, err)
		assert.Equal(t, "url1", result.Config.Projects["a"])
		assert.Equal(t, "url2", result.Config.Projects["b"])
	})

	t.Run("merges multiple overlays in order", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"), `{"projects":{"a":"url1"}}`)
		writeFile(t, filepath.Join(dir, ".gogo.devops"), `{"projects":{"b":"url2"}}`)
		writeFile(t, filepath.Join(dir, ".gogo.staging"), `{"projects":{"c":"url3","a":"url-override"}}`)

		result, err := ReadMetaConfig(dir, []string{".gogo.devops", ".gogo.staging"})
		require.NoError(t, err)
		assert.Equal(t, "url-override", result.Config.Projects["a"])
		assert.Equal(t, "url2", result.Config.Projects["b"])
		assert.Equal(t, "url3", result.Config.Projects["c"])
	})

	t.Run("resolves overlay paths relative to metaDir", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"), `{"projects":{"a":"url1"}}`)
		writeFile(t, filepath.Join(dir, ".gogo.devops"), `{"projects":{"b":"url2"}}`)
		sub := filepath.Join(dir, "sub")
		require.NoError(t, os.MkdirAll(sub, 0o755))

		result, err := ReadMetaConfig(sub, []string{".gogo.devops"})
		require.NoError(t, err)
		assert.Equal(t, "url1", result.Config.Projects["a"])
		assert.Equal(t, "url2", result.Config.Projects["b"])
	})

	t.Run("throws ConfigError for missing overlay", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"), `{"projects":{}}`)

		_, err := ReadMetaConfig(dir, []string{".gogo.nonexistent"})
		assert.Error(t, err)
		var configErr *ConfigError
		assert.ErrorAs(t, err, &configErr)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("uses global overlay files when param not provided", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"), `{"projects":{"a":"url1"}}`)
		writeFile(t, filepath.Join(dir, ".gogo.devops"), `{"projects":{"b":"url2"}}`)

		SetOverlayFiles([]string{".gogo.devops"})
		defer SetOverlayFiles(nil)

		result, err := ReadMetaConfig(dir, nil)
		require.NoError(t, err)
		assert.Equal(t, "url1", result.Config.Projects["a"])
		assert.Equal(t, "url2", result.Config.Projects["b"])
	})

	t.Run("preserves primary format when merging overlays", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo.yaml"), "projects:\n  a: url1\n")
		writeFile(t, filepath.Join(dir, ".gogo.devops"), `{"projects":{"b":"url2"}}`)

		result, err := ReadMetaConfig(dir, []string{".gogo.devops"})
		require.NoError(t, err)
		assert.Equal(t, FormatYAML, result.Format)
		assert.Equal(t, dir, result.MetaDir)
	})
}

func TestCommandConfigJSON(t *testing.T) {
	t.Run("parses string command", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"), `{"projects":{},"commands":{"build":"npm run build"}}`)

		result, err := ReadMetaConfig(dir, nil)
		require.NoError(t, err)
		assert.Equal(t, "npm run build", result.Config.Commands["build"].Cmd)
	})

	t.Run("parses object command", func(t *testing.T) {
		dir := t.TempDir()
		concurrency := 2
		_ = concurrency
		writeFile(t, filepath.Join(dir, ".gogo"), `{"projects":{},"commands":{"test":{"cmd":"npm test","parallel":true,"concurrency":2,"description":"Run tests","includeOnly":["api","web"]}}}`)

		result, err := ReadMetaConfig(dir, nil)
		require.NoError(t, err)
		cmd := result.Config.Commands["test"]
		assert.Equal(t, "npm test", cmd.Cmd)
		assert.NotNil(t, cmd.Parallel)
		assert.True(t, *cmd.Parallel)
		assert.NotNil(t, cmd.Concurrency)
		assert.Equal(t, 2, *cmd.Concurrency)
		assert.Equal(t, "Run tests", cmd.Description)
		assert.Equal(t, []string{"api", "web"}, cmd.IncludeOnly)
	})

	t.Run("parses mixed commands", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"), `{"projects":{},"commands":{"build":"npm run build","test":{"cmd":"npm test","parallel":true}}}`)

		result, err := ReadMetaConfig(dir, nil)
		require.NoError(t, err)
		assert.Equal(t, "npm run build", result.Config.Commands["build"].Cmd)
		assert.Equal(t, "npm test", result.Config.Commands["test"].Cmd)
	})

	t.Run("rejects command without cmd field", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"), `{"projects":{},"commands":{"build":{"parallel":true}}}`)

		_, err := ReadMetaConfig(dir, nil)
		assert.Error(t, err)
	})
}

func TestGetCommand(t *testing.T) {
	t.Run("returns false for non-existent command", func(t *testing.T) {
		config := MetaConfig{Projects: map[string]string{}, Ignore: []string{}}
		_, ok := GetCommand(config, "build")
		assert.False(t, ok)
	})

	t.Run("returns false when commands nil", func(t *testing.T) {
		config := MetaConfig{Projects: map[string]string{}, Ignore: []string{}}
		_, ok := GetCommand(config, "build")
		assert.False(t, ok)
	})

	t.Run("returns command", func(t *testing.T) {
		config := MetaConfig{
			Projects: map[string]string{}, Ignore: []string{},
			Commands: map[string]CommandConfig{"build": {Cmd: "npm run build"}},
		}
		cmd, ok := GetCommand(config, "build")
		assert.True(t, ok)
		assert.Equal(t, "npm run build", cmd.Cmd)
	})
}

func TestListCommands(t *testing.T) {
	t.Run("returns nil for no commands", func(t *testing.T) {
		config := MetaConfig{Projects: map[string]string{}, Ignore: []string{}}
		assert.Nil(t, ListCommands(config))
	})

	t.Run("returns sorted command list", func(t *testing.T) {
		config := MetaConfig{
			Projects: map[string]string{}, Ignore: []string{},
			Commands: map[string]CommandConfig{
				"build": {Cmd: "npm run build"},
				"test":  {Cmd: "npm test", Description: "Run all tests"},
			},
		}
		entries := ListCommands(config)
		assert.Len(t, entries, 2)
		assert.Equal(t, "build", entries[0].Name)
		assert.Equal(t, "npm run build", entries[0].Command.Cmd)
		assert.Equal(t, "test", entries[1].Name)
		assert.Equal(t, "Run all tests", entries[1].Command.Description)
	})
}

func TestIsSafeProjectPath(t *testing.T) {
	safe := []string{"libs/api", "a/b/c", "a/../b", "api"}
	for _, p := range safe {
		assert.True(t, IsSafeProjectPath(p), "expected safe: %q", p)
	}
	unsafe := []string{"../x", "../../x", "/abs", "a/../../b", "..", "", ".", "/"}
	for _, p := range unsafe {
		assert.False(t, IsSafeProjectPath(p), "expected unsafe: %q", p)
	}
}

func TestValidateRejectsUnsafeProjectPath(t *testing.T) {
	err := Validate(MetaConfig{Projects: map[string]string{"../evil": "git@x:o/r.git"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid project path")

	require.NoError(t, Validate(MetaConfig{Projects: map[string]string{"libs/api": "git@x:o/r.git"}}))
}

func TestOverlayFilesState(t *testing.T) {
	SetOverlayFiles(nil)
	assert.Nil(t, GetOverlayFiles())

	SetOverlayFiles([]string{".gogo.devops", ".gogo.staging"})
	assert.Equal(t, []string{".gogo.devops", ".gogo.staging"}, GetOverlayFiles())
	SetOverlayFiles(nil)
}

func groupConfig() MetaConfig {
	return MetaConfig{
		Projects: map[string]string{
			"libs/a": "urlA",
			"libs/b": "urlB",
			"libs/c": "urlC",
			"libs/d": "urlD",
		},
		Ignore: []string{},
		Groups: map[string][]string{
			"foo": {"libs/a", "libs/b"},
			"bar": {"libs/b", "libs/c"},
		},
	}
}

func TestResolveGroups(t *testing.T) {
	cfg := groupConfig()

	t.Run("resolves a single group", func(t *testing.T) {
		paths, err := ResolveGroups(cfg, []string{"foo"})
		require.NoError(t, err)
		assert.Equal(t, []string{"libs/a", "libs/b"}, paths)
	})

	t.Run("unions multiple groups and deduplicates", func(t *testing.T) {
		paths, err := ResolveGroups(cfg, []string{"foo", "bar"})
		require.NoError(t, err)
		assert.Equal(t, []string{"libs/a", "libs/b", "libs/c"}, paths)
	})

	t.Run("returns nil for no group names", func(t *testing.T) {
		paths, err := ResolveGroups(cfg, nil)
		require.NoError(t, err)
		assert.Nil(t, paths)
	})

	t.Run("errors on unknown group and lists available ones", func(t *testing.T) {
		_, err := ResolveGroups(cfg, []string{"nope"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `unknown group "nope"`)
		assert.Contains(t, err.Error(), "bar, foo")
	})

	t.Run("errors when no groups are defined", func(t *testing.T) {
		_, err := ResolveGroups(MetaConfig{Projects: map[string]string{"a": "url"}}, []string{"foo"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "No groups are defined")
	})

	t.Run("errors on group member that is not a project", func(t *testing.T) {
		bad := groupConfig()
		bad.Groups["broken"] = []string{"libs/missing"}
		_, err := ResolveGroups(bad, []string{"broken"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `unknown project "libs/missing"`)
	})

	t.Run("matches members written with a redundant path prefix", func(t *testing.T) {
		cleaned := groupConfig()
		cleaned.Groups["dotted"] = []string{"./libs/d"}
		paths, err := ResolveGroups(cleaned, []string{"dotted"})
		require.NoError(t, err)
		assert.Equal(t, []string{"libs/d"}, paths)
	})
}

func TestGetGroupNames(t *testing.T) {
	assert.Equal(t, []string{"bar", "foo"}, GetGroupNames(groupConfig()))
	assert.Empty(t, GetGroupNames(MetaConfig{Projects: map[string]string{}}))
}

func TestValidateGroups(t *testing.T) {
	t.Run("accepts valid groups", func(t *testing.T) {
		require.NoError(t, Validate(groupConfig()))
	})

	t.Run("rejects empty group name", func(t *testing.T) {
		cfg := groupConfig()
		cfg.Groups[""] = []string{"libs/a"}
		err := Validate(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "group names must not be empty")
	})

	t.Run("rejects group without members", func(t *testing.T) {
		cfg := groupConfig()
		cfg.Groups["empty"] = []string{}
		err := Validate(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must contain at least one project")
	})

	t.Run("rejects empty member path", func(t *testing.T) {
		cfg := groupConfig()
		cfg.Groups["blank"] = []string{" "}
		err := Validate(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "project paths must not be empty")
	})

	t.Run("does not check group members across files", func(t *testing.T) {
		// A single overlay may group projects declared in the base config.
		cfg := MetaConfig{
			Projects: map[string]string{},
			Groups:   map[string][]string{"foo": {"libs/a"}},
		}
		require.NoError(t, Validate(cfg))
	})
}

func TestValidateReferences(t *testing.T) {
	t.Run("accepts a consistent config", func(t *testing.T) {
		cfg := groupConfig()
		cfg.Commands = map[string]CommandConfig{"deploy": {Cmd: "make deploy", Groups: []string{"foo"}}}
		assert.Empty(t, ValidateReferences(cfg))
	})

	t.Run("rejects group member that is not a project", func(t *testing.T) {
		cfg := groupConfig()
		cfg.Groups["broken"] = []string{"libs/missing"}
		problems := ValidateReferences(cfg)
		require.Len(t, problems, 1)
		assert.Contains(t, problems[0].Error(), `group "broken": unknown project "libs/missing"`)
	})

	t.Run("rejects command referencing an unknown group", func(t *testing.T) {
		cfg := groupConfig()
		cfg.Commands = map[string]CommandConfig{"deploy": {Cmd: "make deploy", Groups: []string{"nope"}}}
		problems := ValidateReferences(cfg)
		require.Len(t, problems, 1)
		assert.Contains(t, problems[0].Error(), `command "deploy": unknown group "nope"`)
	})

	t.Run("reports every problem, ordered by group and command name", func(t *testing.T) {
		cfg := groupConfig()
		cfg.Groups["broken"] = []string{"libs/missing", "libs/gone"}
		cfg.Groups["also-broken"] = []string{"libs/nowhere"}
		cfg.Commands = map[string]CommandConfig{
			"deploy":  {Cmd: "make deploy", Groups: []string{"nope"}},
			"analyze": {Cmd: "make analyze", Groups: []string{"foo", "missing"}},
		}

		problems := ValidateReferences(cfg)
		messages := make([]string, len(problems))
		for i, problem := range problems {
			messages[i] = problem.Error()
		}
		assert.Equal(t, []string{
			`group "also-broken": unknown project "libs/nowhere"`,
			`group "broken": unknown project "libs/missing"`,
			`group "broken": unknown project "libs/gone"`,
			`command "analyze": unknown group "missing"`,
			`command "deploy": unknown group "nope"`,
		}, messages)
	})
}

func TestGroupsInConfigFiles(t *testing.T) {
	t.Run("reads groups from JSON", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"),
			`{"projects":{"libs/a":"urlA","libs/b":"urlB"},"groups":{"foo":["libs/a","libs/b"]}}`)

		result, err := ReadMetaConfig(dir, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"libs/a", "libs/b"}, result.Config.Groups["foo"])
	})

	t.Run("reads groups from YAML", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo.yaml"),
			"projects:\n  libs/a: urlA\n  libs/b: urlB\ngroups:\n  foo:\n    - libs/a\n    - libs/b\n")

		result, err := ReadMetaConfig(dir, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"libs/a", "libs/b"}, result.Config.Groups["foo"])
	})

	t.Run("tolerates a stale group reference until the group is used", func(t *testing.T) {
		// The referenced project may live in an overlay that is not loaded, so
		// reading must not fail; using the group must.
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"),
			`{"projects":{"libs/a":"urlA"},"groups":{"foo":["libs/nope"]}}`)

		result, err := ReadMetaConfig(dir, nil)
		require.NoError(t, err)

		_, err = ResolveGroups(result.Config, []string{"foo"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), `unknown project "libs/nope"`)

		assert.NotEmpty(t, ValidateReferences(result.Config))
	})

	t.Run("an overlay may group projects from the base config", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"), `{"projects":{"libs/a":"urlA","libs/b":"urlB"}}`)
		writeFile(t, filepath.Join(dir, ".gogo.devops"), `{"projects":{},"groups":{"foo":["libs/a"]}}`)

		result, err := ReadMetaConfig(dir, []string{".gogo.devops"})
		require.NoError(t, err)
		assert.Equal(t, []string{"libs/a"}, result.Config.Groups["foo"])
	})

	t.Run("round-trips groups through write and read", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, WriteMetaConfig(dir, groupConfig(), FormatJSON))

		result, err := ReadMetaConfig(dir, nil)
		require.NoError(t, err)
		assert.Equal(t, groupConfig().Groups, result.Config.Groups)
	})

	t.Run("parses command group references", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".gogo"),
			`{"projects":{"libs/a":"urlA"},"groups":{"foo":["libs/a"]},"commands":{"deploy":{"cmd":"make deploy","groups":["foo"]}}}`)

		result, err := ReadMetaConfig(dir, nil)
		require.NoError(t, err)
		assert.Equal(t, []string{"foo"}, result.Config.Commands["deploy"].Groups)
	})

	t.Run("keeps a command with groups an object when marshaled", func(t *testing.T) {
		data, err := json.Marshal(CommandConfig{Cmd: "make deploy", Groups: []string{"foo"}})
		require.NoError(t, err)
		assert.JSONEq(t, `{"cmd":"make deploy","groups":["foo"]}`, string(data))
	})
}

func TestMergeConfigsGroups(t *testing.T) {
	base := MetaConfig{
		Projects: map[string]string{"a": "urlA"},
		Groups:   map[string][]string{"foo": {"a"}, "bar": {"a"}},
	}
	overlay := MetaConfig{
		Projects: map[string]string{"b": "urlB"},
		Groups:   map[string][]string{"foo": {"b"}, "baz": {"b"}},
	}

	merged := MergeConfigs(base, overlay)
	assert.Equal(t, []string{"b"}, merged.Groups["foo"], "overlay wins per group name")
	assert.Equal(t, []string{"a"}, merged.Groups["bar"])
	assert.Equal(t, []string{"b"}, merged.Groups["baz"])

	t.Run("does not mutate inputs", func(t *testing.T) {
		assert.Equal(t, []string{"a"}, base.Groups["foo"])
	})

	t.Run("stays nil when neither side has groups", func(t *testing.T) {
		merged := MergeConfigs(MetaConfig{Projects: map[string]string{}}, MetaConfig{Projects: map[string]string{}})
		assert.Nil(t, merged.Groups)
	})
}

func TestProjectChangesKeepGroupsConsistent(t *testing.T) {
	t.Run("AddProject preserves groups", func(t *testing.T) {
		updated := AddProject(groupConfig(), "libs/e", "urlE")
		assert.Equal(t, []string{"libs/a", "libs/b"}, updated.Groups["foo"])
	})

	t.Run("RemoveProject drops the project from groups", func(t *testing.T) {
		updated := RemoveProject(groupConfig(), "libs/a")
		assert.Equal(t, []string{"libs/b"}, updated.Groups["foo"])
		assert.Equal(t, []string{"libs/b", "libs/c"}, updated.Groups["bar"])
		require.NoError(t, Validate(updated))
	})

	t.Run("RemoveProject drops groups left empty", func(t *testing.T) {
		cfg := MetaConfig{
			Projects: map[string]string{"a": "urlA", "b": "urlB"},
			Groups:   map[string][]string{"solo": {"a"}, "both": {"a", "b"}},
		}
		updated := RemoveProject(cfg, "a")
		_, ok := updated.Groups["solo"]
		assert.False(t, ok)
		assert.Equal(t, []string{"b"}, updated.Groups["both"])
		require.NoError(t, Validate(updated))
	})

	t.Run("RemoveProject leaves no command pointing at a dropped group", func(t *testing.T) {
		cfg := MetaConfig{
			Projects: map[string]string{"a": "urlA", "b": "urlB"},
			Groups:   map[string][]string{"solo": {"a"}, "both": {"a", "b"}},
			Commands: map[string]CommandConfig{
				"deploy": {Cmd: "make deploy", Groups: []string{"solo", "both"}},
				"build":  {Cmd: "make build"},
			},
		}
		updated := RemoveProject(cfg, "a")
		assert.Equal(t, []string{"both"}, updated.Commands["deploy"].Groups)
		assert.Empty(t, updated.Commands["build"].Groups)
		require.NoError(t, Validate(updated))
		assert.Empty(t, ValidateReferences(updated))

		// The original config is untouched.
		assert.Equal(t, []string{"solo", "both"}, cfg.Commands["deploy"].Groups)
	})
}

func TestValidateReferencesAgainstAlsoKnown(t *testing.T) {
	base := MetaConfig{
		Projects: map[string]string{"api": "urlA"},
		Groups:   map[string][]string{"deploy": {"infra/tf"}},
	}
	overlay := MetaConfig{
		Projects: map[string]string{"infra/tf": "urlT"},
		Groups:   map[string][]string{"infra": {"infra/tf"}},
	}

	t.Run("a group member declared by an extra config is accepted", func(t *testing.T) {
		assert.NotEmpty(t, ValidateReferences(base))
		assert.Empty(t, ValidateReferences(base, overlay))
	})

	t.Run("a command group declared by an extra config is accepted", func(t *testing.T) {
		cfg := MetaConfig{
			Projects: map[string]string{"api": "urlA"},
			Commands: map[string]CommandConfig{"deploy": {Cmd: "make deploy", Groups: []string{"infra"}}},
		}
		assert.NotEmpty(t, ValidateReferences(cfg))
		assert.Empty(t, ValidateReferences(cfg, overlay))
	})

	t.Run("a reference no config satisfies is still reported", func(t *testing.T) {
		problems := ValidateReferences(base, MetaConfig{Projects: map[string]string{"other": "urlO"}})
		require.Len(t, problems, 1)
		assert.Contains(t, problems[0].Error(), `group "deploy": unknown project "infra/tf"`)
	})

	t.Run("extra configs do not redefine the members of a group under check", func(t *testing.T) {
		// The overlay's "deploy" is consistent, the base's is not. Which
		// projects "deploy" contains must be read from the config under check,
		// so the broken member is still reported.
		shadowing := MetaConfig{
			Projects: map[string]string{"api": "urlA"},
			Groups:   map[string][]string{"deploy": {"api"}},
		}
		problems := ValidateReferences(base, shadowing)
		require.Len(t, problems, 1)
		assert.Contains(t, problems[0].Error(), `unknown project "infra/tf"`)
	})

	t.Run("passing no extra config keeps the single-config behavior", func(t *testing.T) {
		assert.Equal(t, ValidateReferences(base), ValidateReferences(base))
		assert.Empty(t, ValidateReferences(groupConfig()))
	})
}
