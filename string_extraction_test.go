package jqfunc

import (
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestStringExtractionBehavior(t *testing.T) {
	t.Run("string input with string result returns string directly", func(t *testing.T) {
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

		testFunc, exists := functions["get_name"]
		require.True(t, exists, "get_name function should exist")

		// Test with JSON string input that returns a string
		jsonInput := `{"name": "Alice", "age": 30}`
		args := []cty.Value{cty.StringVal(jsonInput)}

		result, err := testFunc.Call(args)
		require.NoError(t, err, "Function call should succeed")

		// Should return the string directly, not JSON-encoded
		assert.Equal(t, cty.String, result.Type(), "Result should be a string")
		assert.Equal(t, "Alice", result.AsString(), "Should return the extracted string directly")
		// Verify it's NOT JSON-encoded (which would be "\"Alice\"")
		assert.NotEqual(t, "\"Alice\"", result.AsString(), "Should not be JSON-encoded")
	})

	t.Run("string input with number result returns JSON string", func(t *testing.T) {
		hclCode := `
jqfunction "get_age" {
    params = []
    query = ".age"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		testFunc := functions["get_age"]

		// Test with JSON string input that returns a number
		jsonInput := `{"name": "Alice", "age": 30}`
		args := []cty.Value{cty.StringVal(jsonInput)}

		result, err := testFunc.Call(args)
		require.NoError(t, err, "Function call should succeed")

		// Should return JSON-encoded string since result is not a string
		assert.Equal(t, cty.String, result.Type(), "Result should be a string")
		assert.Equal(t, "30", result.AsString(), "Should return the number as JSON string")
	})

	t.Run("string input with object result returns JSON string", func(t *testing.T) {
		hclCode := `
jqfunction "get_user" {
    params = []
    query = "{name: .name, age: .age}"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		testFunc := functions["get_user"]

		// Test with JSON string input that returns an object
		jsonInput := `{"name": "Alice", "age": 30, "city": "NYC"}`
		args := []cty.Value{cty.StringVal(jsonInput)}

		result, err := testFunc.Call(args)
		require.NoError(t, err, "Function call should succeed")

		// Should return JSON-encoded string since result is an object
		assert.Equal(t, cty.String, result.Type(), "Result should be a string")
		resultStr := result.AsString()

		// Should be valid JSON containing the expected fields
		assert.Contains(t, resultStr, `"name":"Alice"`, "Should contain name field")
		assert.Contains(t, resultStr, `"age":30`, "Should contain age field")
		// Should start and end with braces (JSON object)
		assert.True(t, resultStr[0] == '{' && resultStr[len(resultStr)-1] == '}',
			"Should be a JSON object string")
	})

	t.Run("string input with array result returns JSON string", func(t *testing.T) {
		hclCode := `
jqfunction "get_array" {
    params = []
    query = "[.name, .age]"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		testFunc := functions["get_array"]

		// Test with JSON string input that returns an array
		jsonInput := `{"name": "Alice", "age": 30}`
		args := []cty.Value{cty.StringVal(jsonInput)}

		result, err := testFunc.Call(args)
		require.NoError(t, err, "Function call should succeed")

		// Should return JSON-encoded string since result is an array
		assert.Equal(t, cty.String, result.Type(), "Result should be a string")
		resultStr := result.AsString()

		// Should be valid JSON array
		assert.True(t, resultStr[0] == '[' && resultStr[len(resultStr)-1] == ']',
			"Should be a JSON array string")
		assert.Contains(t, resultStr, `"Alice"`, "Should contain the name")
		assert.Contains(t, resultStr, `30`, "Should contain the age")
	})

	t.Run("cty input with string result returns cty string", func(t *testing.T) {
		hclCode := `
jqfunction "get_name_cty" {
    params = []
    query = ".name"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		testFunc := functions["get_name_cty"]

		// Test with cty object input
		ctyInput := cty.ObjectVal(map[string]cty.Value{
			"name": cty.StringVal("Bob"),
			"age":  cty.NumberIntVal(25),
		})
		args := []cty.Value{ctyInput}

		result, err := testFunc.Call(args)
		require.NoError(t, err, "Function call should succeed")

		// Should return cty string directly (not JSON-encoded)
		assert.Equal(t, cty.String, result.Type(), "Result should be a string")
		assert.Equal(t, "Bob", result.AsString(), "Should return the extracted string")
	})

	t.Run("comparison of string vs cty input behavior", func(t *testing.T) {
		hclCode := `
jqfunction "extract_name" {
    params = []
    query = ".name"
}
`
		parser := hclparse.NewParser()
		file, diags := parser.ParseHCL([]byte(hclCode), "test.hcl")
		require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

		functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
		require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

		testFunc := functions["extract_name"]

		// Test with JSON string input
		jsonInput := `{"name": "Charlie", "age": 35}`
		stringResult, err := testFunc.Call([]cty.Value{cty.StringVal(jsonInput)})
		require.NoError(t, err, "String input should succeed")

		// Test with cty object input
		ctyInput := cty.ObjectVal(map[string]cty.Value{
			"name": cty.StringVal("Charlie"),
			"age":  cty.NumberIntVal(35),
		})
		ctyResult, err := testFunc.Call([]cty.Value{ctyInput})
		require.NoError(t, err, "Cty input should succeed")

		// Both should return the same string value
		assert.Equal(t, cty.String, stringResult.Type(), "String input result should be string")
		assert.Equal(t, cty.String, ctyResult.Type(), "Cty input result should be string")
		assert.Equal(t, "Charlie", stringResult.AsString(), "String input should extract name")
		assert.Equal(t, "Charlie", ctyResult.AsString(), "Cty input should extract name")

		// Both results should be equal
		assert.True(t, stringResult.RawEquals(ctyResult), "Both approaches should yield the same result")
	})
}
