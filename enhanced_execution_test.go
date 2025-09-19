package jqfunc

import (
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

func TestEnhancedJqFunctionExecution(t *testing.T) {
	hclCode := `
jqfunction "extract_field" {
    params = [field]
    query = ".[$field]"
}

jqfunction "multiply_values" {
    params = [multiplier]
    query = "map(. * $multiplier)"
}

jqfunction "add_field" {
    params = [key, value]
    query = ". + {($key): $value}"
}

jqfunction "get_length" {
    params = []
    query = "length"
}
`

	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL([]byte(hclCode), "enhanced.hcl")
	require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

	functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
	require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)
	require.Len(t, functions, 4, "Should have four functions")

	t.Run("string input (legacy behavior)", func(t *testing.T) {
		extractField, exists := functions["extract_field"]
		require.True(t, exists, "extract_field function should exist")

		// Test with JSON string input
		jsonInput := `{"name": "Alice", "age": 30}`
		args := []cty.Value{
			cty.StringVal(jsonInput),
			cty.StringVal("name"),
		}

		result, err := extractField.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.Equal(t, cty.String, result.Type(), "Result should be string for string input")
		assert.Equal(t, `Alice`, result.AsString(), "Should extract the name field directly (not JSON-encoded)")
	})

	t.Run("cty object input (new behavior)", func(t *testing.T) {
		extractField, exists := functions["extract_field"]
		require.True(t, exists, "extract_field function should exist")

		// Test with cty object input
		objInput := cty.ObjectVal(map[string]cty.Value{
			"name": cty.StringVal("Bob"),
			"age":  cty.NumberIntVal(25),
		})
		args := []cty.Value{
			objInput,
			cty.StringVal("name"),
		}

		result, err := extractField.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.Equal(t, cty.String, result.Type(), "Result should be string")
		assert.Equal(t, "Bob", result.AsString(), "Should extract the name field")
	})

	t.Run("cty list input", func(t *testing.T) {
		multiplyValues, exists := functions["multiply_values"]
		require.True(t, exists, "multiply_values function should exist")

		// Test with cty list input
		listInput := cty.ListVal([]cty.Value{
			cty.NumberIntVal(1),
			cty.NumberIntVal(2),
			cty.NumberIntVal(3),
		})
		args := []cty.Value{
			listInput,
			cty.NumberIntVal(3),
		}

		result, err := multiplyValues.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.Equal(t, cty.List(cty.Number), result.Type(), "Result should be list of numbers")

		// Check the result values
		resultList := result.AsValueSlice()
		require.Len(t, resultList, 3, "Should have 3 elements")
		assert.True(t, resultList[0].RawEquals(cty.NumberIntVal(3)), "First element should be 3")
		assert.True(t, resultList[1].RawEquals(cty.NumberIntVal(6)), "Second element should be 6")
		assert.True(t, resultList[2].RawEquals(cty.NumberIntVal(9)), "Third element should be 9")
	})

	t.Run("adding field to cty object", func(t *testing.T) {
		addField, exists := functions["add_field"]
		require.True(t, exists, "add_field function should exist")

		// Test adding field to cty object
		objInput := cty.ObjectVal(map[string]cty.Value{
			"name": cty.StringVal("Charlie"),
		})
		args := []cty.Value{
			objInput,
			cty.StringVal("age"),
			cty.NumberIntVal(35),
		}

		result, err := addField.Call(args)
		require.NoError(t, err, "Function call should succeed")

		// Result should be an object with both fields
		assert.True(t, result.Type().IsObjectType(), "Result should be an object")
		resultObj := result.AsValueMap()
		assert.Equal(t, "Charlie", resultObj["name"].AsString(), "Should preserve existing field")
		assert.True(t, resultObj["age"].RawEquals(cty.NumberIntVal(35)), "Should add new field")
	})

	t.Run("get length of different types", func(t *testing.T) {
		getLength, exists := functions["get_length"]
		require.True(t, exists, "get_length function should exist")

		// Test with cty list
		listInput := cty.ListVal([]cty.Value{
			cty.StringVal("a"),
			cty.StringVal("b"),
			cty.StringVal("c"),
		})
		args := []cty.Value{listInput}

		result, err := getLength.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.True(t, result.RawEquals(cty.NumberIntVal(3)), "List should have length 3")

		// Test with cty object
		objInput := cty.ObjectVal(map[string]cty.Value{
			"a": cty.StringVal("value1"),
			"b": cty.StringVal("value2"),
		})
		args = []cty.Value{objInput}

		result, err = getLength.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.True(t, result.RawEquals(cty.NumberIntVal(2)), "Object should have length 2")

		// Test with string (should still work as JSON)
		stringInput := cty.StringVal(`[1, 2, 3, 4]`)
		args = []cty.Value{stringInput}

		result, err = getLength.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.Equal(t, cty.String, result.Type(), "String input should return string result")
		assert.Equal(t, "4", result.AsString(), "JSON array should have length 4")
	})

	t.Run("complex nested operations", func(t *testing.T) {
		// Create a complex nested structure
		nestedInput := cty.ObjectVal(map[string]cty.Value{
			"users": cty.ListVal([]cty.Value{
				cty.ObjectVal(map[string]cty.Value{
					"name": cty.StringVal("Alice"),
					"age":  cty.NumberIntVal(30),
				}),
				cty.ObjectVal(map[string]cty.Value{
					"name": cty.StringVal("Bob"),
					"age":  cty.NumberIntVal(25),
				}),
			}),
		})

		extractField, exists := functions["extract_field"]
		require.True(t, exists, "extract_field function should exist")

		args := []cty.Value{
			nestedInput,
			cty.StringVal("users"),
		}

		result, err := extractField.Call(args)
		require.NoError(t, err, "Function call should succeed")

		// Should extract the users array
		assert.True(t, result.Type().IsListType(), "Result should be a list")
		usersList := result.AsValueSlice()
		require.Len(t, usersList, 2, "Should have 2 users")

		// Check first user
		firstUser := usersList[0].AsValueMap()
		assert.Equal(t, "Alice", firstUser["name"].AsString(), "First user should be Alice")
		assert.True(t, firstUser["age"].RawEquals(cty.NumberIntVal(30)), "Alice should be 30")
	})

	t.Run("string multiplication (jq feature)", func(t *testing.T) {
		multiplyValues, exists := functions["multiply_values"]
		require.True(t, exists, "multiply_values function should exist")

		// In jq, multiplying strings by numbers repeats the string
		listInput := cty.ListVal([]cty.Value{
			cty.StringVal("hello"),
			cty.StringVal("world"),
		})
		args := []cty.Value{
			listInput,
			cty.NumberIntVal(2),
		}

		result, err := multiplyValues.Call(args)
		require.NoError(t, err, "String multiplication should work in jq")

		// Should return a list of repeated strings
		assert.True(t, result.Type().IsListType(), "Result should be a list")
		resultList := result.AsValueSlice()
		require.Len(t, resultList, 2, "Should have 2 elements")
		assert.Equal(t, "hellohello", resultList[0].AsString(), "Should repeat 'hello' twice")
		assert.Equal(t, "worldworld", resultList[1].AsString(), "Should repeat 'world' twice")
	})

	t.Run("empty object handling", func(t *testing.T) {
		extractField, exists := functions["extract_field"]
		require.True(t, exists, "extract_field function should exist")

		// Test with empty object input
		emptyObj := cty.ObjectVal(map[string]cty.Value{})
		args := []cty.Value{
			emptyObj,
			cty.StringVal("nonexistent"),
		}

		result, err := extractField.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.True(t, result.IsNull(), "Result should be null for nonexistent field")
	})
}

// TestMixedInputTypes demonstrates functions working with both string and cty inputs
func TestMixedInputTypes(t *testing.T) {
	hclCode := `
jqfunction "transform_data" {
    params = [operation]
    query = "if $operation == \"double\" then map(. * 2) elif $operation == \"keys\" then keys else . end"
}
`

	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL([]byte(hclCode), "mixed.hcl")
	require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

	functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
	require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

	transformData, exists := functions["transform_data"]
	require.True(t, exists, "transform_data function should exist")

	t.Run("same operation on string vs cty input", func(t *testing.T) {
		// Test with JSON string
		stringArgs := []cty.Value{
			cty.StringVal(`[1, 2, 3]`),
			cty.StringVal("double"),
		}

		stringResult, err := transformData.Call(stringArgs)
		require.NoError(t, err, "String input should work")
		assert.Equal(t, cty.String, stringResult.Type(), "String input should return string")
		assert.Equal(t, "[2,4,6]", stringResult.AsString(), "Should double the values")

		// Test with cty list
		ctyArgs := []cty.Value{
			cty.ListVal([]cty.Value{
				cty.NumberIntVal(1),
				cty.NumberIntVal(2),
				cty.NumberIntVal(3),
			}),
			cty.StringVal("double"),
		}

		ctyResult, err := transformData.Call(ctyArgs)
		require.NoError(t, err, "Cty input should work")
		assert.True(t, ctyResult.Type().IsListType(), "Cty input should return list")

		resultList := ctyResult.AsValueSlice()
		expected := []cty.Value{
			cty.NumberIntVal(2),
			cty.NumberIntVal(4),
			cty.NumberIntVal(6),
		}

		for i, expectedVal := range expected {
			assert.True(t, resultList[i].RawEquals(expectedVal), "Element %d should be %v", i, expectedVal)
		}
	})
}
