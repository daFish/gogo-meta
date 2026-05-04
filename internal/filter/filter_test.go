package filter

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApply(t *testing.T) {
	dirs := []string{"src/app", "src/lib", "docs", "tools/cli"}

	t.Run("includeOnly filters to matching dirs", func(t *testing.T) {
		result := Apply(dirs, Options{IncludeOnly: []string{"src/app", "docs"}})
		assert.Equal(t, []string{"src/app", "docs"}, result)
	})

	t.Run("includeOnly matches basename", func(t *testing.T) {
		result := Apply(dirs, Options{IncludeOnly: []string{"app"}})
		assert.Equal(t, []string{"src/app"}, result)
	})

	t.Run("excludeOnly removes matching dirs", func(t *testing.T) {
		result := Apply(dirs, Options{ExcludeOnly: []string{"docs"}})
		assert.Equal(t, []string{"src/app", "src/lib", "tools/cli"}, result)
	})

	t.Run("excludeOnly matches basename", func(t *testing.T) {
		result := Apply(dirs, Options{ExcludeOnly: []string{"cli"}})
		assert.Equal(t, []string{"src/app", "src/lib", "docs"}, result)
	})

	t.Run("includePattern filters by regex", func(t *testing.T) {
		result := Apply(dirs, Options{IncludePattern: regexp.MustCompile(`^src/`)})
		assert.Equal(t, []string{"src/app", "src/lib"}, result)
	})

	t.Run("excludePattern removes by regex", func(t *testing.T) {
		result := Apply(dirs, Options{ExcludePattern: regexp.MustCompile(`^docs`)})
		assert.Equal(t, []string{"src/app", "src/lib", "tools/cli"}, result)
	})

	t.Run("combined filters apply sequentially", func(t *testing.T) {
		result := Apply(dirs, Options{
			ExcludeOnly:    []string{"docs"},
			IncludePattern: regexp.MustCompile(`^src/`),
		})
		assert.Equal(t, []string{"src/app", "src/lib"}, result)
	})

	t.Run("empty options returns all dirs", func(t *testing.T) {
		result := Apply(dirs, Options{})
		assert.Equal(t, dirs, result)
	})

	t.Run("does not mutate input", func(t *testing.T) {
		original := []string{"a", "b", "c"}
		input := make([]string, len(original))
		copy(input, original)
		Apply(input, Options{IncludeOnly: []string{"a"}})
		assert.Equal(t, original, input)
	})
}

func TestParseFilterList(t *testing.T) {
	t.Run("splits comma-separated values", func(t *testing.T) {
		assert.Equal(t, []string{"a", "b", "c"}, ParseFilterList("a,b,c"))
	})

	t.Run("trims whitespace", func(t *testing.T) {
		assert.Equal(t, []string{"a", "b"}, ParseFilterList(" a , b "))
	})

	t.Run("filters empty strings", func(t *testing.T) {
		assert.Equal(t, []string{"a", "b"}, ParseFilterList("a,,b,"))
	})

	t.Run("returns nil for empty input", func(t *testing.T) {
		assert.Nil(t, ParseFilterList(""))
	})
}

func TestParseFilterPattern(t *testing.T) {
	t.Run("compiles valid regex", func(t *testing.T) {
		re, err := ParseFilterPattern(`^src/`)
		require.NoError(t, err)
		assert.True(t, re.MatchString("src/app"))
	})

	t.Run("returns nil for empty input", func(t *testing.T) {
		re, err := ParseFilterPattern("")
		require.NoError(t, err)
		assert.Nil(t, re)
	})

	t.Run("returns error for invalid regex", func(t *testing.T) {
		_, err := ParseFilterPattern("[invalid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid regex pattern")
	})
}

func TestCreateFilterOptions(t *testing.T) {
	t.Run("creates options from strings", func(t *testing.T) {
		opts, err := CreateFilterOptions("a,b", "c", `^src/`, `test$`)
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, opts.IncludeOnly)
		assert.Equal(t, []string{"c"}, opts.ExcludeOnly)
		assert.NotNil(t, opts.IncludePattern)
		assert.NotNil(t, opts.ExcludePattern)
	})

	t.Run("handles empty strings", func(t *testing.T) {
		opts, err := CreateFilterOptions("", "", "", "")
		require.NoError(t, err)
		assert.Nil(t, opts.IncludeOnly)
		assert.Nil(t, opts.ExcludeOnly)
		assert.Nil(t, opts.IncludePattern)
		assert.Nil(t, opts.ExcludePattern)
	})
}

func TestFilterFromLoopRc(t *testing.T) {
	dirs := []string{"src/app", "src/lib", "docs", "examples"}

	t.Run("filters matching dirs", func(t *testing.T) {
		result := FilterFromLoopRc(dirs, []string{"docs", "examples"})
		assert.Equal(t, []string{"src/app", "src/lib"}, result)
	})

	t.Run("returns all dirs when ignore is empty", func(t *testing.T) {
		result := FilterFromLoopRc(dirs, []string{})
		assert.Equal(t, dirs, result)
	})

	t.Run("matches basename", func(t *testing.T) {
		result := FilterFromLoopRc(dirs, []string{"app"})
		assert.Equal(t, []string{"src/lib", "docs", "examples"}, result)
	})
}
