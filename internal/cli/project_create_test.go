package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProjectCreateRejectsUnsafeFolder(t *testing.T) {
	err := runProjectCreate(nil, []string{"../evil", "git@x:o/r.git"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid project folder")
}
