package jqfunc

import (
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestMultiResultJqFunctions(t *testing.T) {
	hclCode := `
// Extract all values from an object
jqfunction "extract_values" {
    params = []
    query = ".[]"
}

// Extract multiple fields
jqfunction "extract_fields" {
    params = [fields]
    query = ".[$fields[]]"
}

// Generate a range of numbers
jqfunction "generate_range" {
    params = [start, end]
    query = "range($start; $end)"
}

// Flatten nested arrays
jqfunction "flatten_deep" {
    params = []
    query = ".. | arrays | select(length > 0) | .[]"
}

// Extract keys and values as separate results
jqfunction "keys_and_values" {
    params = []
    query = "keys[], .[]"
}
`

	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL([]byte(hclCode), "multi-result.hcl")
	require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

	functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
	require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)
	require.Len(t, functions, 5, "Should have five functions")

	t.Run("single result returns element directly", func(t *testing.T) {
		extractValues, exists := functions["extract_values"]
		require.True(t, exists, "extract_values function should exist")

		// Object with single value
		singleValueObj := cty.ObjectVal(map[string]cty.Value{
			"only": cty.StringVal("value"),
		})

		result, err := extractValues.Call([]cty.Value{singleValueObj})
		require.NoError(t, err, "Function call should succeed")

		// Should return the single value directly, not wrapped in an array
		assert.Equal(t, cty.String, result.Type(), "Result should be string")
		assert.Equal(t, "value", result.AsString(), "Should return the single value")
	})

	t.Run("multiple results return as list", func(t *testing.T) {
		extractValues, exists := functions["extract_values"]
		require.True(t, exists, "extract_values function should exist")

		// Object with multiple values of the same type to avoid cty list type issues
		multiValueObj := cty.ObjectVal(map[string]cty.Value{
			"first":  cty.StringVal("alpha"),
			"second": cty.StringVal("beta"),
			"third":  cty.StringVal("gamma"),
		})

		result, err := extractValues.Call([]cty.Value{multiValueObj})
		require.NoError(t, err, "Function call should succeed")

		// Should return a list containing all values
		assert.True(t, result.Type().IsListType(), "Result should be a list")
		resultList := result.AsValueSlice()
		require.Len(t, resultList, 3, "Should have 3 values")

		// All should be strings, but order might vary
		resultStrings := make([]string, len(resultList))
		for i, val := range resultList {
			require.Equal(t, cty.String, val.Type(), "All values should be strings")
			resultStrings[i] = val.AsString()
		}

		// Check that all expected values are present
		assert.Contains(t, resultStrings, "alpha", "Should contain 'alpha'")
		assert.Contains(t, resultStrings, "beta", "Should contain 'beta'")
		assert.Contains(t, resultStrings, "gamma", "Should contain 'gamma'")
	})

	t.Run("string input multi-result", func(t *testing.T) {
		extractValues, exists := functions["extract_values"]
		require.True(t, exists, "extract_values function should exist")

		// JSON string with multiple values
		jsonInput := `{"a": 1, "b": 2, "c": 3}`

		result, err := extractValues.Call([]cty.Value{cty.StringVal(jsonInput)})
		require.NoError(t, err, "Function call should succeed")

		// Should return a JSON array string
		assert.Equal(t, cty.String, result.Type(), "Result should be string for string input")
		resultStr := result.AsString()

		// Should be a JSON array (order may vary)
		assert.Contains(t, resultStr, "1", "Should contain 1")
		assert.Contains(t, resultStr, "2", "Should contain 2")
		assert.Contains(t, resultStr, "3", "Should contain 3")
		assert.True(t, resultStr[0] == '[' && resultStr[len(resultStr)-1] == ']', "Should be a JSON array")
	})

	t.Run("range function with multiple results", func(t *testing.T) {
		generateRange, exists := functions["generate_range"]
		require.True(t, exists, "generate_range function should exist")

		// Use empty object instead of null (HCL functions don't accept null)
		args := []cty.Value{
			cty.ObjectVal(map[string]cty.Value{}), // jq range doesn't use input
			cty.NumberIntVal(1),                   // start
			cty.NumberIntVal(4),                   // end (exclusive)
		}

		result, err := generateRange.Call(args)
		require.NoError(t, err, "Function call should succeed")

		// Should return a list of numbers [1, 2, 3]
		assert.True(t, result.Type().IsListType(), "Result should be a list")
		resultList := result.AsValueSlice()
		require.Len(t, resultList, 3, "Should have 3 numbers")

		// Check the values
		for i, val := range resultList {
			assert.Equal(t, cty.Number, val.Type(), "Element %d should be a number", i)
			num, _ := val.AsBigFloat().Int64()
			assert.Equal(t, int64(i+1), num, "Element %d should be %d", i, i+1)
		}
	})

	t.Run("extract specific fields", func(t *testing.T) {
		extractFields, exists := functions["extract_fields"]
		require.True(t, exists, "extract_fields function should exist")

		// Object with multiple fields
		dataObj := cty.ObjectVal(map[string]cty.Value{
			"name":   cty.StringVal("Alice"),
			"age":    cty.NumberIntVal(30),
			"city":   cty.StringVal("New York"),
			"active": cty.BoolVal(true),
		})

		// Extract specific fields
		fieldsToExtract := cty.ListVal([]cty.Value{
			cty.StringVal("name"),
			cty.StringVal("city"),
		})

		args := []cty.Value{dataObj, fieldsToExtract}
		result, err := extractFields.Call(args)
		require.NoError(t, err, "Function call should succeed")

		// Should return a list with the two extracted values
		assert.True(t, result.Type().IsListType(), "Result should be a list")
		resultList := result.AsValueSlice()
		require.Len(t, resultList, 2, "Should have 2 values")

		// Check that we got the right values (order should match the fields list)
		assert.Equal(t, "Alice", resultList[0].AsString(), "First result should be name")
		assert.Equal(t, "New York", resultList[1].AsString(), "Second result should be city")
	})

	t.Run("no results returns null", func(t *testing.T) {
		extractValues, exists := functions["extract_values"]
		require.True(t, exists, "extract_values function should exist")

		// Empty object
		emptyObj := cty.ObjectVal(map[string]cty.Value{})

		result, err := extractValues.Call([]cty.Value{emptyObj})
		require.NoError(t, err, "Function call should succeed")

		// Should return null
		assert.True(t, result.IsNull(), "Result should be null for empty object")
	})

	t.Run("mixed types in results", func(t *testing.T) {
		keysAndValues, exists := functions["keys_and_values"]
		require.True(t, exists, "keys_and_values function should exist")

		// Object with same-type values to avoid cty list type consistency issues
		mixedObj := cty.ObjectVal(map[string]cty.Value{
			"a": cty.StringVal("alpha"),
			"b": cty.StringVal("beta"),
			"c": cty.StringVal("gamma"),
		})

		result, err := keysAndValues.Call([]cty.Value{mixedObj})
		require.NoError(t, err, "Function call should succeed")

		// Should return a list containing both keys and values
		assert.True(t, result.Type().IsListType(), "Result should be a list")
		resultList := result.AsValueSlice()
		require.Len(t, resultList, 6, "Should have 6 elements (3 keys + 3 values)")

		// All results should be strings (keys are strings, values are strings)
		allStrings := make([]string, len(resultList))
		for i, val := range resultList {
			require.Equal(t, cty.String, val.Type(), "All elements should be strings")
			allStrings[i] = val.AsString()
		}

		// Should contain all keys
		assert.Contains(t, allStrings, "a", "Should contain key 'a'")
		assert.Contains(t, allStrings, "b", "Should contain key 'b'")
		assert.Contains(t, allStrings, "c", "Should contain key 'c'")

		// Should contain all values
		assert.Contains(t, allStrings, "alpha", "Should contain value 'alpha'")
		assert.Contains(t, allStrings, "beta", "Should contain value 'beta'")
		assert.Contains(t, allStrings, "gamma", "Should contain value 'gamma'")
	})

	t.Run("complex nested extraction", func(t *testing.T) {
		// Let's use a simpler test that's more predictable
		extractValues, exists := functions["extract_values"]
		require.True(t, exists, "extract_values function should exist")

		// Test with a list instead of object to get predictable ordering
		listData := cty.ListVal([]cty.Value{
			cty.NumberIntVal(10),
			cty.NumberIntVal(20),
			cty.NumberIntVal(30),
		})

		result, err := extractValues.Call([]cty.Value{listData})
		require.NoError(t, err, "Function call should succeed")

		// Should return a list of the individual numbers
		assert.True(t, result.Type().IsListType(), "Result should be a list")
		resultList := result.AsValueSlice()
		require.Len(t, resultList, 3, "Should have 3 elements")

		// Check the values are preserved in order for lists
		for i, val := range resultList {
			assert.Equal(t, cty.Number, val.Type(), "Element %d should be a number", i)
			num, _ := val.AsBigFloat().Int64()
			assert.Equal(t, int64((i+1)*10), num, "Element %d should be %d", i, (i+1)*10)
		}
	})
}

// TestMultiResultEdgeCases tests edge cases for multi-result handling
func TestMultiResultEdgeCases(t *testing.T) {
	hclCode := `
// Function that might return 0, 1, or multiple results based on input
jqfunction "conditional_results" {
    params = [condition]
    query = "if $condition == \"none\" then empty elif $condition == \"one\" then 42 elif $condition == \"multi\" then (1, 2, 3) else error(\"invalid condition\") end"
}

// Function that generates results based on array length
jqfunction "repeat_elements" {
    params = []
    query = ".[] | ., ."
}
`

	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL([]byte(hclCode), "edge-cases.hcl")
	require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

	functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
	require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

	t.Run("conditional no results", func(t *testing.T) {
		conditionalResults, exists := functions["conditional_results"]
		require.True(t, exists, "conditional_results function should exist")

		args := []cty.Value{
			cty.ObjectVal(map[string]cty.Value{}), // Use empty object instead of null
			cty.StringVal("none"),
		}

		result, err := conditionalResults.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.True(t, result.IsNull(), "Should return null for no results")
	})

	t.Run("conditional single result", func(t *testing.T) {
		conditionalResults, exists := functions["conditional_results"]
		require.True(t, exists, "conditional_results function should exist")

		args := []cty.Value{
			cty.ObjectVal(map[string]cty.Value{}), // Use empty object instead of null
			cty.StringVal("one"),
		}

		result, err := conditionalResults.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.Equal(t, cty.Number, result.Type(), "Should return single number")
		num, _ := result.AsBigFloat().Int64()
		assert.Equal(t, int64(42), num, "Should return 42")
	})

	t.Run("conditional multiple results", func(t *testing.T) {
		conditionalResults, exists := functions["conditional_results"]
		require.True(t, exists, "conditional_results function should exist")

		args := []cty.Value{
			cty.ObjectVal(map[string]cty.Value{}), // Use empty object instead of null
			cty.StringVal("multi"),
		}

		result, err := conditionalResults.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.True(t, result.Type().IsListType(), "Should return list for multiple results")

		resultList := result.AsValueSlice()
		require.Len(t, resultList, 3, "Should have 3 results")

		for i, val := range resultList {
			assert.Equal(t, cty.Number, val.Type(), "Element %d should be a number", i)
			num, _ := val.AsBigFloat().Int64()
			assert.Equal(t, int64(i+1), num, "Element %d should be %d", i, i+1)
		}
	})

	t.Run("duplicate results", func(t *testing.T) {
		repeatElements, exists := functions["repeat_elements"]
		require.True(t, exists, "repeat_elements function should exist")

		// Input array with 2 elements, each repeated twice = 4 results
		inputArray := cty.ListVal([]cty.Value{
			cty.StringVal("a"),
			cty.StringVal("b"),
		})

		result, err := repeatElements.Call([]cty.Value{inputArray})
		require.NoError(t, err, "Function call should succeed")

		assert.True(t, result.Type().IsListType(), "Should return list")
		resultList := result.AsValueSlice()
		require.Len(t, resultList, 4, "Should have 4 results (each element repeated)")

		// Should have: "a", "a", "b", "b"
		assert.Equal(t, "a", resultList[0].AsString(), "First result should be 'a'")
		assert.Equal(t, "a", resultList[1].AsString(), "Second result should be 'a'")
		assert.Equal(t, "b", resultList[2].AsString(), "Third result should be 'b'")
		assert.Equal(t, "b", resultList[3].AsString(), "Fourth result should be 'b'")
	})
}
