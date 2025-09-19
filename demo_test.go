package jqfunc

import (
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zclconf/go-cty/cty"
)

// TestFullDemo demonstrates the complete functionality of the jq function extension
func TestFullDemo(t *testing.T) {
	hclCode := `
// Extract a field from JSON
jqfunction "get_field" {
    params = [field]
    query = ".[$field]"
}

// Transform an array
jqfunction "multiply_array" {
    params = [multiplier]
    query = "map(. * $multiplier)"
}

// Complex query combining JSON input and parameters
jqfunction "filter_and_transform" {
    params = [min_age, bonus]
    query = "[.[] | select(.age >= $min_age) | .salary = (.salary + $bonus)]"
}

// Simple query without parameters
jqfunction "extract_names" {
    params = []
    query = "[.[].name]"
}
`

	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL([]byte(hclCode), "demo.hcl")
	require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

	// Parse and compile all functions
	functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
	require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)
	require.Len(t, functions, 4, "Should have four functions")

	t.Run("get_field function", func(t *testing.T) {
		getField, exists := functions["get_field"]
		require.True(t, exists, "get_field function should exist")

		jsonData := `{"name": "Alice", "age": 30, "city": "New York"}`
		args := []cty.Value{
			cty.StringVal(jsonData),
			cty.StringVal("name"),
		}

		result, err := getField.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.Equal(t, `Alice`, result.AsString(), "Should extract the name field directly (not JSON-encoded)")

		// Test with different field
		args[1] = cty.StringVal("age")
		result, err = getField.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.Equal(t, "30", result.AsString(), "Should extract the age field")
	})

	t.Run("multiply_array function", func(t *testing.T) {
		multiplyArray, exists := functions["multiply_array"]
		require.True(t, exists, "multiply_array function should exist")

		jsonData := `[1, 2, 3, 4, 5]`
		args := []cty.Value{
			cty.StringVal(jsonData),
			cty.NumberIntVal(3),
		}

		result, err := multiplyArray.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.Equal(t, "[3,6,9,12,15]", result.AsString(), "Should multiply each element by 3")
	})

	t.Run("filter_and_transform function", func(t *testing.T) {
		filterAndTransform, exists := functions["filter_and_transform"]
		require.True(t, exists, "filter_and_transform function should exist")

		jsonData := `[
			{"name": "Alice", "age": 25, "salary": 50000},
			{"name": "Bob", "age": 35, "salary": 60000},
			{"name": "Charlie", "age": 22, "salary": 45000}
		]`
		args := []cty.Value{
			cty.StringVal(jsonData),
			cty.NumberIntVal(25),   // min_age
			cty.NumberIntVal(5000), // bonus
		}

		result, err := filterAndTransform.Call(args)
		require.NoError(t, err, "Function call should succeed")

		// The result should be an array with Alice and Bob (age >= 25) with bonuses added
		resultStr := result.AsString()
		assert.Contains(t, resultStr, `"name":"Alice"`, "Should include Alice")
		assert.Contains(t, resultStr, `"name":"Bob"`, "Should include Bob")
		assert.Contains(t, resultStr, `"salary":55000`, "Alice's salary should be 55000")
		assert.Contains(t, resultStr, `"salary":65000`, "Bob's salary should be 65000")
		assert.NotContains(t, resultStr, `"name":"Charlie"`, "Should not include Charlie (age < 25)")
	})

	t.Run("extract_names function", func(t *testing.T) {
		extractNames, exists := functions["extract_names"]
		require.True(t, exists, "extract_names function should exist")

		jsonData := `[
			{"name": "Alice", "age": 25},
			{"name": "Bob", "age": 35},
			{"name": "Charlie", "age": 22}
		]`
		args := []cty.Value{
			cty.StringVal(jsonData),
		}

		result, err := extractNames.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.Equal(t, `["Alice","Bob","Charlie"]`, result.AsString(), "Should extract all names")
	})

	t.Run("error handling with runtime errors", func(t *testing.T) {
		getField, exists := functions["get_field"]
		require.True(t, exists, "get_field function should exist")

		// Try to access a field that doesn't exist - should return null
		jsonData := `{"name": "Alice", "age": 30}`
		args := []cty.Value{
			cty.StringVal(jsonData),
			cty.StringVal("nonexistent"),
		}

		result, err := getField.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.Equal(t, "null", result.AsString(), "Should return null for nonexistent field")
	})

	t.Run("float parameter types", func(t *testing.T) {
		// Test with float parameter
		multiplyArray, exists := functions["multiply_array"]
		require.True(t, exists, "multiply_array function should exist")

		jsonData := `[1, 2, 3, 4]`
		args := []cty.Value{
			cty.StringVal(jsonData),
			cty.NumberFloatVal(2.5), // float multiplier
		}

		result, err := multiplyArray.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.Equal(t, "[2.5,5,7.5,10]", result.AsString(), "Should multiply each element by 2.5")
	})

	t.Run("direct cty value input (new feature)", func(t *testing.T) {
		// Test with direct cty values instead of JSON strings
		getField, exists := functions["get_field"]
		require.True(t, exists, "get_field function should exist")

		// Use a cty object directly
		userData := cty.ObjectVal(map[string]cty.Value{
			"name":   cty.StringVal("David"),
			"age":    cty.NumberIntVal(28),
			"active": cty.BoolVal(true),
		})

		args := []cty.Value{
			userData,
			cty.StringVal("name"),
		}

		result, err := getField.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.Equal(t, cty.String, result.Type(), "Result should be a string")
		assert.Equal(t, "David", result.AsString(), "Should extract the name field")

		// Test with different field
		args[1] = cty.StringVal("active")
		result, err = getField.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.Equal(t, cty.Bool, result.Type(), "Result should be a boolean")
		assert.True(t, result.True(), "Should extract the active field as true")
	})

	t.Run("cty list operations", func(t *testing.T) {
		multiplyArray, exists := functions["multiply_array"]
		require.True(t, exists, "multiply_array function should exist")

		// Use a cty list directly
		numbers := cty.ListVal([]cty.Value{
			cty.NumberIntVal(10),
			cty.NumberIntVal(20),
			cty.NumberIntVal(30),
		})

		args := []cty.Value{
			numbers,
			cty.NumberFloatVal(0.1), // 10% of each value
		}

		result, err := multiplyArray.Call(args)
		require.NoError(t, err, "Function call should succeed")
		assert.True(t, result.Type().IsListType(), "Result should be a list")

		resultList := result.AsValueSlice()
		require.Len(t, resultList, 3, "Should have 3 elements")
		assert.True(t, resultList[0].RawEquals(cty.NumberFloatVal(1.0)), "10 * 0.1 = 1.0")
		assert.True(t, resultList[1].RawEquals(cty.NumberFloatVal(2.0)), "20 * 0.1 = 2.0")
		assert.True(t, resultList[2].RawEquals(cty.NumberFloatVal(3.0)), "30 * 0.1 = 3.0")
	})
}

// TestRealWorldExample demonstrates a realistic use case
func TestRealWorldExample(t *testing.T) {
	hclCode := `
// Extract user information with default values
jqfunction "extract_user_info" {
    params = [default_role]
    query = "{name: .name, email: .email, role: (.role // $default_role), active: (.active // true)}"
}

// Calculate total price with tax
jqfunction "calculate_total" {
    params = [tax_rate]
    query = "{subtotal: .subtotal, tax: (.subtotal * $tax_rate), total: (.subtotal * (1 + $tax_rate))}"
}
`

	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL([]byte(hclCode), "real-world.hcl")
	require.False(t, diags.HasErrors(), "HCL parsing should succeed: %s", diags)

	functions, _, diags := DecodeJqFunctions(file.Body, "jqfunction")
	require.False(t, diags.HasErrors(), "Function decoding should succeed: %s", diags)

	t.Run("user info extraction", func(t *testing.T) {
		extractUserInfo, exists := functions["extract_user_info"]
		require.True(t, exists, "extract_user_info function should exist")

		// User with all fields
		userData := `{"name": "Alice Johnson", "email": "alice@example.com", "role": "admin", "active": true}`
		args := []cty.Value{
			cty.StringVal(userData),
			cty.StringVal("user"), // default role
		}

		result, err := extractUserInfo.Call(args)
		require.NoError(t, err, "Function call should succeed")

		resultStr := result.AsString()
		assert.Contains(t, resultStr, `"name":"Alice Johnson"`)
		assert.Contains(t, resultStr, `"email":"alice@example.com"`)
		assert.Contains(t, resultStr, `"role":"admin"`)
		assert.Contains(t, resultStr, `"active":true`)

		// User with missing fields (should use defaults)
		userData = `{"name": "Bob Smith", "email": "bob@example.com"}`
		args[0] = cty.StringVal(userData)

		result, err = extractUserInfo.Call(args)
		require.NoError(t, err, "Function call should succeed")

		resultStr = result.AsString()
		assert.Contains(t, resultStr, `"role":"user"`, "Should use default role")
		assert.Contains(t, resultStr, `"active":true`, "Should use default active status")
	})

	t.Run("price calculation", func(t *testing.T) {
		calculateTotal, exists := functions["calculate_total"]
		require.True(t, exists, "calculate_total function should exist")

		priceData := `{"subtotal": 100.00}`
		args := []cty.Value{
			cty.StringVal(priceData),
			cty.NumberFloatVal(0.08), // 8% tax rate
		}

		result, err := calculateTotal.Call(args)
		require.NoError(t, err, "Function call should succeed")

		resultStr := result.AsString()
		assert.Contains(t, resultStr, `"subtotal":100`)
		assert.Contains(t, resultStr, `"tax":8`)
		assert.Contains(t, resultStr, `"total":108`)
	})
}
