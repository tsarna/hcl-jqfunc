package jqfunc

import (
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeJqFunctions(t *testing.T) {
	t.Run("single function", func(t *testing.T) {
		hclCode := `
jqfunction "test_func" {
    params = [x, y]
    query = ".x + .y"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, remainingBody, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		require.Len(t, functions, 1, "Should extract one function")

		// Check that the function was created
		testFunc, exists := functions["test_func"]
		require.True(t, exists, "test_func should be created")
		assert.NotNil(t, testFunc, "Function should not be nil")

		// Remaining body should be empty
		content, _, _ := remainingBody.PartialContent(&hcl.BodySchema{})
		assert.Empty(t, content.Blocks, "No blocks should remain")
	})

	t.Run("multiple functions", func(t *testing.T) {
		hclCode := `
jqfunction "add" {
    params = [a, b]
    query = ".a + .b"
}

jqfunction "multiply" {
    params = [x, y, z]
    query = ".x * .y * .z"
}

jqfunction "extract_name" {
    params = [obj]
    query = ".obj.name"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		require.Len(t, functions, 3, "Should extract three functions")

		// Check that all functions were created
		_, exists := functions["add"]
		assert.True(t, exists, "add function should be created")

		_, exists = functions["multiply"]
		assert.True(t, exists, "multiply function should be created")

		_, exists = functions["extract_name"]
		assert.True(t, exists, "extract_name function should be created")
	})

	t.Run("function with no params", func(t *testing.T) {
		hclCode := `
jqfunction "constant" {
    params = []
    query = "42"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		require.Len(t, functions, 1, "Should extract one function")

		// Check that the function was created
		_, exists := functions["constant"]
		assert.True(t, exists, "constant function should be created")
	})

	t.Run("mixed with other blocks", func(t *testing.T) {
		hclCode := `
some_other_block "test" {
    value = "hello"
}

jqfunction "process" {
    params = [input]
    query = ".input | keys"
}

another_block {
    data = "world"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, remainingBody, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		require.Len(t, functions, 1, "Should extract one function")

		// Check that the function was created
		_, exists := functions["process"]
		assert.True(t, exists, "process function should be created")

		// Check that other blocks remain
		content, _, _ := remainingBody.PartialContent(&hcl.BodySchema{
			Blocks: []hcl.BlockHeaderSchema{
				{Type: "some_other_block", LabelNames: []string{"name"}},
				{Type: "another_block"},
			},
		})
		assert.Len(t, content.Blocks, 2, "Other blocks should remain")
		assert.Equal(t, "some_other_block", content.Blocks[0].Type)
		assert.Equal(t, "another_block", content.Blocks[1].Type)
	})
}

func TestDecodeJqFunctions_Errors(t *testing.T) {
	t.Run("missing function name", func(t *testing.T) {
		hclCode := `
jqfunction {
    params = ["x"]
    query = ".x"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed")

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.True(t, diags.HasErrors(), "Should have errors for missing function name")
		assert.Empty(t, functions, "No functions should be extracted")
	})

	t.Run("missing query", func(t *testing.T) {
		hclCode := `
jqfunction "test" {
    params = [x]
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed")

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.True(t, diags.HasErrors(), "Should have errors for missing query")
		assert.Contains(t, diags.Error(), "required argument")
		assert.Empty(t, functions, "No functions should be extracted")
	})

	t.Run("empty query", func(t *testing.T) {
		hclCode := `
jqfunction "test" {
    params = [x]
    query = ""
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed")

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.True(t, diags.HasErrors(), "Should have errors for empty query")
		assert.Contains(t, diags.Error(), "Missing query")
		assert.Empty(t, functions, "No functions should be extracted")
	})

	t.Run("too many labels", func(t *testing.T) {
		hclCode := `
jqfunction "test" "extra" {
    params = [x]
    query = ".x"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed")

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.True(t, diags.HasErrors(), "Should have errors for too many labels")
		assert.Contains(t, diags.Error(), "Extraneous label")
		assert.Empty(t, functions, "No functions should be extracted")
	})

	t.Run("string parameters rejected", func(t *testing.T) {
		hclCode := `
jqfunction "test" {
    params = ["x", "y"]  # strings should be rejected
    query = ".x + .y"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed")

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.True(t, diags.HasErrors(), "Should have errors for string parameters")
		assert.Contains(t, diags.Error(), "bare identifiers")
		assert.Empty(t, functions, "No functions should be extracted")
	})

	t.Run("complex parameter expressions rejected", func(t *testing.T) {
		hclCode := `
jqfunction "test" {
    params = [x.y, z]  # complex expressions should be rejected
    query = ".x + .y"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed")

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.True(t, diags.HasErrors(), "Should have errors for complex parameter expressions")
		assert.Contains(t, diags.Error(), "bare identifiers")
		assert.Empty(t, functions, "No functions should be extracted")
	})
}

// validateJqFunction validates a jq function by checking if it's properly compiled (test helper)
func validateJqFunction(jqFunc *JqFunction) hcl.Diagnostics {
	var diags hcl.Diagnostics

	// Check if the function is properly compiled
	if jqFunc.CompiledQuery == nil {
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Function not compiled",
			Detail:   "JQ function was not properly compiled",
			Subject:  &jqFunc.Range,
		})
	}

	return diags
}

func TestValidateJqFunction(t *testing.T) {
	t.Run("properly compiled function", func(t *testing.T) {
		hclCode := `
jqfunction "test" {
    params = [x]
    query = "$x | keys"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)
		require.Len(t, functions, 1, "Should have one function")

		// Check that the function exists
		_, exists := functions["test"]
		assert.True(t, exists, "test function should be created")
	})

	t.Run("function without compiled query", func(t *testing.T) {
		jqFunc := &JqFunction{
			Name:          "test",
			Params:        []string{"x"},
			Query:         "$x | keys",
			CompiledQuery: nil, // Not compiled
			Range:         hcl.Range{},
		}

		diags := validateJqFunction(jqFunc)
		assert.True(t, diags.HasErrors(), "Function without compiled query should have errors")
		assert.Contains(t, diags.Error(), "not compiled")
	})
}
