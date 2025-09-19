package jqfunc

import (
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAndCompile_Integration(t *testing.T) {
	t.Run("complete workflow", func(t *testing.T) {
		hclCode := `
jqfunction "add" {
    params = [a, b]
    query = "$a + $b"
}

jqfunction "extract_field" {
    params = [field]
    query = ".[$field]"
}

jqfunction "constant" {
    params = []
    query = "42"
}

jqfunction "transform_list" {
    params = [multiplier]
    query = "map(. * $multiplier)"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		// Parse and compile the function definitions
		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding and compilation should succeed: %s", diags)
		require.Len(t, functions, 4, "Should extract and compile four functions")

		// Check that all functions were created
		expectedFunctions := []string{"add", "extract_field", "constant", "transform_list"}
		for _, name := range expectedFunctions {
			fn, exists := functions[name]
			assert.True(t, exists, "%s function should be created", name)
			assert.NotNil(t, fn, "%s function should not be nil", name)
		}
	})

	t.Run("mixed valid and invalid functions", func(t *testing.T) {
		hclCode := `
jqfunction "good" {
    params = [x]
    query = "$x * 2"
}

jqfunction "bad_syntax" {
    params = [y]
    query = "$y | invalid_func("
}

jqfunction "undefined_var" {
    params = [z]
    query = "$z + $unknown"
}

jqfunction "another_good" {
    params = []
    query = "length"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		// Parse and compile the function definitions
		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.True(t, diags.HasErrors(), "Should have compilation errors")

		// Should compile the valid ones and skip the invalid ones
		require.Len(t, functions, 2, "Should compile two valid functions")

		// Check that good functions are compiled
		_, exists := functions["good"]
		assert.True(t, exists, "good function should be compiled")

		_, exists = functions["another_good"]
		assert.True(t, exists, "another_good function should be compiled")

		// Check that bad functions are not in compiled list
		_, exists = functions["bad_syntax"]
		assert.False(t, exists, "bad_syntax function should not be compiled")

		_, exists = functions["undefined_var"]
		assert.False(t, exists, "undefined_var function should not be compiled")

		// Verify error messages
		errorMsg := diags.Error()
		assert.Contains(t, errorMsg, "Invalid jq query",
			"Should contain parsing errors, got: %s", errorMsg)
	})

	t.Run("different block types", func(t *testing.T) {
		hclCode := `
transform "uppercase" {
    params = [text]
    query = "$text | ascii_upcase"
}

processor "filter_data" {
    params = [condition]
    query = "map(select($condition))"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		// Parse transform functions
		transformFuncs, _, diags := DecodeJqFunctions(file.Body, "transform")
		require.False(t, diags.HasErrors(), "Transform function decoding should succeed: %s", diags)
		require.Len(t, transformFuncs, 1, "Should extract one transform function")

		// Parse processor functions
		processorFuncs, _, diags := DecodeJqFunctions(file.Body, "processor")
		require.False(t, diags.HasErrors(), "Processor function decoding should succeed: %s", diags)
		require.Len(t, processorFuncs, 1, "Should extract one processor function")

		// Both should already be compiled
		totalFunctions := len(transformFuncs) + len(processorFuncs)
		require.Equal(t, 2, totalFunctions, "Should have both functions")

		// Check transform functions
		_, exists := transformFuncs["uppercase"]
		assert.True(t, exists, "uppercase function should be created")

		// Check processor functions
		_, exists = processorFuncs["filter_data"]
		assert.True(t, exists, "filter_data function should be created")
	})
}
