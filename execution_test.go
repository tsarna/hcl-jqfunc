package jqfunc

import (
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestJqFunctionExecution(t *testing.T) {
	t.Run("simple query without parameters", func(t *testing.T) {
		hclCode := `
jqfunction "get_name" {
    params = []
    query = ".name"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)
		require.Len(t, functions, 1, "Should have one function")

		getName, exists := functions["get_name"]
		require.True(t, exists, "get_name function should exist")

		// Test the function with JSON input
		jsonInput := `{"name": "Alice", "age": 30}`
		args := []cty.Value{cty.StringVal(jsonInput)}

		result, err := getName.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.Equal(t, cty.String, result.Type(), "Result should be string")
		assert.Equal(t, `Alice`, result.AsString(), "Should extract the name directly (not JSON-encoded)")
	})

	t.Run("query with parameters", func(t *testing.T) {
		hclCode := `
jqfunction "add_numbers" {
    params = [x, y]
    query = "$x + $y"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		addNumbers, exists := functions["add_numbers"]
		require.True(t, exists, "add_numbers function should exist")

		// Test the function
		jsonInput := `null` // We don't need the JSON input for this query
		args := []cty.Value{
			cty.StringVal(jsonInput),
			cty.NumberIntVal(5),
			cty.NumberIntVal(3),
		}

		result, err := addNumbers.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.Equal(t, cty.String, result.Type(), "Result should be string")
		assert.Equal(t, "8", result.AsString(), "Should add the numbers")
	})

	t.Run("query using both JSON input and parameters", func(t *testing.T) {
		hclCode := `
jqfunction "extract_field" {
    params = [field]
    query = ".[$field]"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		extractField, exists := functions["extract_field"]
		require.True(t, exists, "extract_field function should exist")

		// Test the function
		jsonInput := `{"name": "Bob", "age": 25, "city": "New York"}`
		args := []cty.Value{
			cty.StringVal(jsonInput),
			cty.StringVal("city"),
		}

		result, err := extractField.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.Equal(t, cty.String, result.Type(), "Result should be string")
		assert.Equal(t, `New York`, result.AsString(), "Should extract the specified field directly (not JSON-encoded)")
	})

	t.Run("error handling - invalid JSON", func(t *testing.T) {
		hclCode := `
jqfunction "get_name" {
    params = []
    query = ".name"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		getName, exists := functions["get_name"]
		require.True(t, exists, "get_name function should exist")

		// Test with invalid JSON
		invalidJSON := `{"name": "Alice", "age":}`
		args := []cty.Value{cty.StringVal(invalidJSON)}

		_, err := getName.Call(args)
		require.Error(t, err, "Function call should fail with invalid JSON")
		assert.Contains(t, err.Error(), "invalid JSON input", "Error should mention invalid JSON")
	})

	t.Run("error handling - wrong argument count", func(t *testing.T) {
		hclCode := `
jqfunction "add_numbers" {
    params = [x, y]
    query = "$x + $y"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		addNumbers, exists := functions["add_numbers"]
		require.True(t, exists, "add_numbers function should exist")

		// Test with wrong number of arguments
		args := []cty.Value{cty.StringVal("null")} // Missing the two parameters

		_, err := addNumbers.Call(args)
		require.Error(t, err, "Function call should fail with wrong argument count")
		assert.Contains(t, err.Error(), "wrong number of arguments", "Error should mention argument count")
	})

	t.Run("complex data types as parameters", func(t *testing.T) {
		hclCode := `
jqfunction "merge_objects" {
    params = [overlay]
    query = ". + $overlay"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		mergeObjects, exists := functions["merge_objects"]
		require.True(t, exists, "merge_objects function should exist")

		// Test with object parameter
		jsonInput := `{"a": 1, "b": 2}`
		overlayObj := cty.ObjectVal(map[string]cty.Value{
			"b": cty.NumberIntVal(3),
			"c": cty.NumberIntVal(4),
		})
		args := []cty.Value{
			cty.StringVal(jsonInput),
			overlayObj,
		}

		result, err := mergeObjects.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.Equal(t, cty.String, result.Type(), "Result should be string")

		// The result should be a merged object (order may vary in JSON)
		resultStr := result.AsString()
		assert.Contains(t, resultStr, `"a":1`, "Should contain original field")
		assert.Contains(t, resultStr, `"b":3`, "Should contain overridden field")
		assert.Contains(t, resultStr, `"c":4`, "Should contain new field")
	})
}
