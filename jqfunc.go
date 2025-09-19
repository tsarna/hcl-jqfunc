package jqfunc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/itchyny/gojq"
	"github.com/tsarna/go2cty2go"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

// JqExecutionError wraps execution errors with source location information
type JqExecutionError struct {
	FunctionName string
	Query        string
	Range        hcl.Range
	Cause        error
}

func (e *JqExecutionError) Error() string {
	return fmt.Sprintf("jq function %s at %s: %v", e.FunctionName, e.Range, e.Cause)
}

func (e *JqExecutionError) Unwrap() error {
	return e.Cause
}

// JqFunction represents a compiled jq function ready for execution
type JqFunction struct {
	Name          string
	Params        []string
	Query         string
	CompiledQuery *gojq.Code
	Range         hcl.Range // For error reporting
}

// DecodeJqFunctions extracts and compiles jq function blocks from HCL bodies, returning HCL functions
// Similar to userfunc.DecodeUserFunctions but for jq functions
func DecodeJqFunctions(body hcl.Body, blockType string) (map[string]function.Function, hcl.Body, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	// Define the schema for the specified block type
	schema := &hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       blockType,
				LabelNames: []string{"name"},
			},
		},
	}

	// Extract jqfunction blocks
	content, remainingBody, contentDiags := body.PartialContent(schema)
	diags = diags.Extend(contentDiags)
	if diags.HasErrors() {
		return nil, nil, diags
	}

	hclFunctions := make(map[string]function.Function)

	// Process each block of the specified type
	for _, block := range content.Blocks {
		if block.Type != blockType {
			continue
		}

		// Ensure we have exactly one label (the function name)
		if len(block.Labels) != 1 {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("Invalid %s block", blockType),
				Detail:   fmt.Sprintf("%s blocks must have exactly one label (the function name)", blockType),
				Subject:  &block.DefRange,
			})
			continue
		}

		// Define schema for the block body to get params and query
		bodySchema := &hcl.BodySchema{
			Attributes: []hcl.AttributeSchema{
				{Name: "params", Required: false},
				{Name: "query", Required: true},
			},
		}

		bodyContent, bodyDiags := block.Body.Content(bodySchema)
		diags = diags.Extend(bodyDiags)
		if bodyDiags.HasErrors() {
			continue
		}

		// Parse params as a list of bare identifiers
		var params []string
		if paramsAttr := bodyContent.Attributes["params"]; paramsAttr != nil {
			// Parse the params expression as a tuple of identifiers
			parsedParams, paramDiags := parseParamsList(paramsAttr.Expr)
			diags = diags.Extend(paramDiags)
			if paramDiags.HasErrors() {
				continue
			}
			params = parsedParams
		}

		// Get query as a string
		var query string
		if queryAttr := bodyContent.Attributes["query"]; queryAttr != nil {
			// Query should be a string literal
			queryVal, queryDiags := queryAttr.Expr.Value(nil)
			diags = diags.Extend(queryDiags)
			if queryDiags.HasErrors() {
				continue
			}
			if queryVal.Type() != cty.String {
				diags = diags.Append(&hcl.Diagnostic{
					Severity: hcl.DiagError,
					Summary:  "Invalid query type",
					Detail:   "Query must be a string literal",
					Subject:  queryAttr.Expr.Range().Ptr(),
				})
				continue
			}
			query = queryVal.AsString()
		}

		// Validate that query is not empty
		if query == "" {
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Missing query",
				Detail:   fmt.Sprintf("%s blocks must specify a 'query' attribute", blockType),
				Subject:  &block.DefRange,
			})
			continue
		}

		// Create and compile the function
		funcDef := &jqFunctionDef{
			Name:   block.Labels[0],
			Params: params,
			Query:  query,
			Range:  block.DefRange,
		}

		compiledFunc, compileDiags := compileJqFunction(funcDef)
		diags = diags.Extend(compileDiags)
		if compileDiags.HasErrors() {
			continue // Skip this function but continue with others
		}

		// Create HCL function from compiled jq function
		hclFunc := createHclFunction(compiledFunc)
		hclFunctions[compiledFunc.Name] = hclFunc
	}

	return hclFunctions, remainingBody, diags
}

// jqFunctionDef represents the raw definition from HCL before compilation (internal type)
type jqFunctionDef struct {
	Name   string
	Params []string
	Query  string
	Range  hcl.Range // For error reporting
}

// parseParamsList parses a params expression as a tuple/list of bare identifiers
func parseParamsList(expr hcl.Expression) ([]string, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	// Try to parse as a tuple expression (list of identifiers)
	if tupleExpr, ok := expr.(*hclsyntax.TupleConsExpr); ok {
		var params []string
		for _, elemExpr := range tupleExpr.Exprs {
			// Each element should be a variable expression (bare identifier)
			if varExpr, ok := elemExpr.(*hclsyntax.ScopeTraversalExpr); ok {
				// Check that it's a simple identifier (no dots)
				if len(varExpr.Traversal) == 1 {
					if step, ok := varExpr.Traversal[0].(hcl.TraverseRoot); ok {
						params = append(params, step.Name)
						continue
					}
				}
			}

			// If we get here, the element is not a simple identifier
			diags = diags.Append(&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "Invalid parameter",
				Detail:   "Parameters must be bare identifiers (e.g., [a, b, c])",
				Subject:  elemExpr.Range().Ptr(),
			})
		}
		return params, diags
	}

	// If it's not a tuple, it might be an empty list or invalid syntax
	diags = diags.Append(&hcl.Diagnostic{
		Severity: hcl.DiagError,
		Summary:  "Invalid params syntax",
		Detail:   "params must be a list of bare identifiers, e.g., params = [a, b, c]",
		Subject:  expr.Range().Ptr(),
	})

	return nil, diags
}

// compileJqFunction compiles a jq function definition with parameter variables (internal function)
func compileJqFunction(funcDef *jqFunctionDef) (*JqFunction, hcl.Diagnostics) {
	var diags hcl.Diagnostics

	// Parse the jq query
	query, err := gojq.Parse(funcDef.Query)
	if err != nil {
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Invalid jq query",
			Detail:   fmt.Sprintf("Failed to parse jq query: %s", err),
			Subject:  &funcDef.Range,
		})
		return nil, diags
	}

	// Create variable names with parameter names prefixed with "$"
	var variables []string
	for _, param := range funcDef.Params {
		variables = append(variables, "$"+param)
	}

	// Compile the query with the parameter variables
	var compiledQuery *gojq.Code
	if len(variables) > 0 {
		compiledQuery, err = gojq.Compile(query, gojq.WithVariables(variables))
	} else {
		// No parameters, compile without variables
		compiledQuery, err = gojq.Compile(query)
	}

	if err != nil {
		diags = diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "Failed to compile jq query",
			Detail:   fmt.Sprintf("Failed to compile jq query with variables: %s", err),
			Subject:  &funcDef.Range,
		})
		return nil, diags
	}

	return &JqFunction{
		Name:          funcDef.Name,
		Params:        funcDef.Params,
		Query:         funcDef.Query,
		CompiledQuery: compiledQuery,
		Range:         funcDef.Range,
	}, diags
}

// createHclFunction creates an HCL function from a compiled jq function
func createHclFunction(jqFunc *JqFunction) function.Function {
	// Build parameter list: first parameter accepts any type, then user-defined parameters (any type)
	params := []function.Parameter{
		{
			Name: "input",
			Type: cty.DynamicPseudoType, // Accept any type
		},
	}

	// Add user-defined parameters (all accept any type)
	for _, paramName := range jqFunc.Params {
		params = append(params, function.Parameter{
			Name: paramName,
			Type: cty.DynamicPseudoType, // Accept any type
		})
	}

	return function.New(&function.Spec{
		Params: params,
		Type:   function.StaticReturnType(cty.DynamicPseudoType), // Can return any type
		Impl: func(args []cty.Value, retType cty.Type) (cty.Value, error) {
			return executeJqFunction(jqFunc, args)
		},
	})
}

// executeJqFunction executes a compiled jq function with the provided arguments
func executeJqFunction(jqFunc *JqFunction, args []cty.Value) (cty.Value, error) {
	// Prepare the input for jq processing
	var jqInput interface{}
	var isStringInput bool

	if args[0].Type() == cty.String {
		// String input: parse as JSON
		jsonStr := args[0].AsString()
		if err := json.Unmarshal([]byte(jsonStr), &jqInput); err != nil {
			return cty.NilVal, &JqExecutionError{
				FunctionName: jqFunc.Name,
				Query:        jqFunc.Query,
				Range:        jqFunc.Range,
				Cause:        fmt.Errorf("invalid JSON input: %v", err),
			}
		}
		isStringInput = true
	} else {
		// Non-string input: convert from cty to Go value
		var err error
		jqInput, err = go2cty2go.CtyToAny(args[0])
		if err != nil {
			return cty.NilVal, &JqExecutionError{
				FunctionName: jqFunc.Name,
				Query:        jqFunc.Query,
				Range:        jqFunc.Range,
				Cause:        fmt.Errorf("failed to convert input: %v", err),
			}
		}
		isStringInput = false
	}

	// Convert remaining arguments from cty to Go values in the same order as parameters
	var variableValues []interface{}
	for i, paramName := range jqFunc.Params {
		argValue, err := go2cty2go.CtyToAny(args[i+1])
		if err != nil {
			return cty.NilVal, &JqExecutionError{
				FunctionName: jqFunc.Name,
				Query:        jqFunc.Query,
				Range:        jqFunc.Range,
				Cause:        fmt.Errorf("failed to convert parameter %s: %v", paramName, err),
			}
		}
		variableValues = append(variableValues, argValue)
	}

	// Execute the compiled jq query with variables as variadic arguments
	var iter gojq.Iter
	if len(variableValues) > 0 {
		iter = jqFunc.CompiledQuery.RunWithContext(context.Background(), jqInput, variableValues...)
	} else {
		iter = jqFunc.CompiledQuery.RunWithContext(context.Background(), jqInput)
	}

	// Collect all results from the iterator
	var results []interface{}
	for {
		result, hasResult := iter.Next()
		if !hasResult {
			break
		}

		// Check for execution error
		if err, ok := result.(error); ok {
			return cty.NilVal, &JqExecutionError{
				FunctionName: jqFunc.Name,
				Query:        jqFunc.Query,
				Range:        jqFunc.Range,
				Cause:        fmt.Errorf("jq execution error: %v", err),
			}
		}

		results = append(results, result)
	}

	// Handle no results
	if len(results) == 0 {
		if isStringInput {
			return cty.StringVal("null"), nil
		} else {
			return cty.NullVal(cty.DynamicPseudoType), nil
		}
	}

	// Determine the final result based on number of results
	var finalResult interface{}
	if len(results) == 1 {
		// Single result: return the element directly
		finalResult = results[0]
	} else {
		// Multiple results: return as array
		finalResult = results
	}

	// Return result based on input type
	if isStringInput {
		// Special case: if the final result is a string, return it directly
		// This is more useful than JSON-encoding it (which would add quotes)
		if str, ok := finalResult.(string); ok {
			return cty.StringVal(str), nil
		}

		// For non-string results: marshal result back to JSON string
		resultJSON, err := json.Marshal(finalResult)
		if err != nil {
			return cty.NilVal, &JqExecutionError{
				FunctionName: jqFunc.Name,
				Query:        jqFunc.Query,
				Range:        jqFunc.Range,
				Cause:        fmt.Errorf("failed to marshal result: %v", err),
			}
		}
		return cty.StringVal(string(resultJSON)), nil
	} else {
		// Non-string input: convert result back to cty value
		ctyResult, err := go2cty2go.AnyToCty(finalResult)
		if err != nil {
			return cty.NilVal, &JqExecutionError{
				FunctionName: jqFunc.Name,
				Query:        jqFunc.Query,
				Range:        jqFunc.Range,
				Cause:        fmt.Errorf("failed to convert result: %v", err),
			}
		}
		return ctyResult, nil
	}
}
