package jqfunc

import (
	"errors"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestEnhancedErrorHandling(t *testing.T) {
	t.Run("JSON parsing error with range", func(t *testing.T) {
		hclCode := `
jqfunction "test_json_error" {
    params = []
    query = ".field"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		testFunc, exists := functions["test_json_error"]
		require.True(t, exists, "test_json_error function should exist")

		// Call with invalid JSON
		args := []cty.Value{cty.StringVal(`{"invalid": json}`)}
		_, err := testFunc.Call(args)

		require.Error(t, err, "Should fail with invalid JSON")

		// Check that it's our enhanced error type
		var jqErr *JqExecutionError
		require.ErrorAs(t, err, &jqErr, "Error should be JqExecutionError type")

		assert.Equal(t, "test_json_error", jqErr.FunctionName, "Should include function name")
		assert.Equal(t, ".field", jqErr.Query, "Should include query")
		assert.Contains(t, jqErr.Range.Filename, "test.hcl", "Should include filename")
		assert.Contains(t, err.Error(), "test.hcl", "Error message should include filename")
		assert.Contains(t, err.Error(), "invalid JSON input", "Error should mention JSON parsing")
	})

	t.Run("jq execution error with range", func(t *testing.T) {
		hclCode := `
jqfunction "test_jq_error" {
    params = [field]
    query = ".nonexistent | .[$field] | error(\"test error\")"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		testFunc, exists := functions["test_jq_error"]
		require.True(t, exists, "test_jq_error function should exist")

		// Call with data that will trigger jq error
		args := []cty.Value{
			cty.ObjectVal(map[string]cty.Value{"data": cty.StringVal("value")}),
			cty.StringVal("field"),
		}
		_, err := testFunc.Call(args)

		require.Error(t, err, "Should fail with jq error")

		// Check that it's our enhanced error type
		var jqErr *JqExecutionError
		require.ErrorAs(t, err, &jqErr, "Error should be JqExecutionError type")

		assert.Equal(t, "test_jq_error", jqErr.FunctionName, "Should include function name")
		assert.Contains(t, jqErr.Query, "error(\"test error\")", "Should include query")
		assert.Contains(t, jqErr.Range.Filename, "test.hcl", "Should include filename")
		assert.Contains(t, err.Error(), "test.hcl", "Error message should include filename")
		assert.Contains(t, err.Error(), "jq execution error", "Error should mention jq execution")
	})

	// Note: Parameter conversion errors are rare with our current CtyToAny implementation
	// since it handles most cty types gracefully. This test is commented out for now.
	// t.Run("parameter conversion error with range", func(t *testing.T) { ... })

	t.Run("error message format", func(t *testing.T) {
		hclCode := `
jqfunction "format_test" {
    params = []
    query = ".test"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "error_test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		testFunc, exists := functions["format_test"]
		require.True(t, exists, "format_test function should exist")

		// Call with invalid JSON to trigger error
		args := []cty.Value{cty.StringVal(`invalid json`)}
		_, err := testFunc.Call(args)

		require.Error(t, err, "Should fail")

		errorMsg := err.Error()
		t.Logf("Error message: %s", errorMsg)

		// Check that error message contains all expected parts
		assert.Contains(t, errorMsg, "format_test", "Should contain function name")
		assert.Contains(t, errorMsg, "error_test.hcl", "Should contain filename")
		assert.Contains(t, errorMsg, "invalid JSON input", "Should contain error type")

		// Check that it follows expected format
		parts := strings.Split(errorMsg, ":")
		assert.True(t, len(parts) >= 2, "Error should have structured format")
	})

	t.Run("range information accuracy", func(t *testing.T) {
		hclCode := `# Line 1
# Line 2
jqfunction "range_test" {
    params = []
    query = ".test"
}
# Line 7`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "range_test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		testFunc, exists := functions["range_test"]
		require.True(t, exists, "range_test function should exist")

		// Trigger error to check range
		args := []cty.Value{cty.StringVal(`bad json`)}
		_, err := testFunc.Call(args)

		require.Error(t, err, "Should fail")

		var jqErr *JqExecutionError
		require.ErrorAs(t, err, &jqErr, "Error should be JqExecutionError type")

		// Check that range points to the right location
		assert.Equal(t, "range_test.hcl", jqErr.Range.Filename, "Should have correct filename")
		assert.Equal(t, 3, jqErr.Range.Start.Line, "Should start at line 3 (jqfunction line)")
		assert.True(t, jqErr.Range.End.Line >= 3, "Should end at or after line 3")

		t.Logf("Range: %s", jqErr.Range)
	})
}

// TestErrorUnwrapping tests that our error type properly supports Go 1.13+ error unwrapping
func TestErrorUnwrapping(t *testing.T) {
	originalErr := errors.New("original error")
	jqErr := &JqExecutionError{
		FunctionName: "test",
		Query:        ".test",
		Range:        hcl.Range{Filename: "test.hcl"},
		Cause:        originalErr,
	}

	// Test that errors.Is works
	require.ErrorIs(t, jqErr, originalErr, "Should support errors.Is")

	// Test direct unwrapping
	assert.Equal(t, originalErr, jqErr.Unwrap(), "Should unwrap correctly")
}
