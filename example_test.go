package jqfunc

import (
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlexibleBlockType(t *testing.T) {
	t.Run("custom block type", func(t *testing.T) {
		hclCode := `
transform "uppercase" {
    params = [text]
    query = ".text | ascii_upcase"
}

transform "get_keys" {
    params = [obj]
    query = ".obj | keys"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, _, diags := DecodeJqFunctions(file.Body, "transform")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		require.Len(t, functions, 2, "Should extract two functions")

		// Check that both functions were created
		_, exists := functions["uppercase"]
		assert.True(t, exists, "uppercase function should be created")

		_, exists = functions["get_keys"]
		assert.True(t, exists, "get_keys function should be created")
	})

	t.Run("mixed block types", func(t *testing.T) {
		hclCode := `
jqfunction "old_style" {
    params = [x]
    query = ".x"
}

transform "new_style" {
    params = [y]
    query = ".y"
}

other_block "ignored" {
    value = "this should be ignored"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		// Extract only transform blocks
		functions, _, diags := DecodeJqFunctions(file.Body, "transform")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		require.Len(t, functions, 1, "Should extract only one transform function")

		// Check that only the transform function was created
		_, exists := functions["new_style"]
		assert.True(t, exists, "new_style function should be created")

		_, exists = functions["old_style"]
		assert.False(t, exists, "old_style function should not be created (wrong block type)")
	})
}
