package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The host constructs runtime.Filter instances internally (from the gRPC
// proto in the golang package, outside runtime to keep it proto-free).
// Extension authors work exclusively with the typed runtime.Filter struct
// and use the companion helpers. These tests guard that contract.

func TestFilterEmpty(t *testing.T) {
	f := Filter{}
	assert.Empty(t, f.Selections, "empty filter has no selections")
}

func TestFilterSingleSelect(t *testing.T) {
	f := NewFilterSelection().Select("type", "manga").Build()

	assert.True(t, HasSelection(f, "type"))
	assert.Equal(t, "manga", FirstSelection(f, "type"))
	assert.Equal(t, []string{"manga"}, SelectionsOf(f, "type"))
	assert.False(t, HasSelection(f, "language"), "undeclared filter has no selection")
	assert.Equal(t, "", FirstSelection(f, "missing"), "missing filter returns empty string")
}

func TestFilterMultiSelect(t *testing.T) {
	f := NewFilterSelection().SelectMany("type", "manga", "bangumi").Select("language", "ja").Build()

	assert.Equal(t, []string{"manga", "bangumi"}, SelectionsOf(f, "type"))
	assert.True(t, HasSelection(f, "type"))
	assert.Equal(t, "manga", FirstSelection(f, "type"))
	assert.Equal(t, "ja", FirstSelection(f, "language"))
}

func TestFilterEmptyValues(t *testing.T) {
	f := NewFilterSelection().SelectMany("type").Build()
	assert.False(t, HasSelection(f, "type"))
	assert.Equal(t, "", FirstSelection(f, "type"))
}

func TestFilterHelpersOnUnset(t *testing.T) {
	f := Filter{}
	assert.False(t, HasSelection(f, "anything"))
	assert.Equal(t, "", FirstSelection(f, "anything"))
	assert.Nil(t, SelectionsOf(f, "anything"))
}
