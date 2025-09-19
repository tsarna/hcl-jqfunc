# HCL JQ Functions

A HashiCorp Configuration Language (HCL) extension that enables user-defined functions written in [JQ](https://jqlang.org/). This package extends HCL with powerful data transformation capabilities using the JQ query language.

## Features

- **JQ Integration**: Write HCL functions using JQ query syntax
- **Flexible Input Types**: Accept both JSON strings and native cty values
- **Smart String Handling**: String results from JSON input are returned directly (not JSON-encoded). You can always use `tojson` in your query if you really want a string containing the qujotes json string.
- **Multi-Result Support**: Handle JQ queries that return multiple values
- **Parameter Support**: Pass HCL function arguments as JQ variables
- **Enhanced Error Reporting**: Detailed error messages with source location information

## Installation

```bash
go get github.com/tsarna/hcl-jqfunc
```

## Quick Start

### Basic Usage

```go
package main

import (
    "fmt"
    "log"
    
    "github.com/hashicorp/hcl/v2/hclparse"
    "github.com/tsarna/hcl-jqfunc"
    "github.com/zclconf/go-cty/cty"
)

func main() {
    // Define HCL with JQ functions
    hclCode := `
    jq "get_name" {
        params = []
        query = ".name"
    }
    
    jq "add_tax" {
        params = [rate]
        query = ".price * (1 + $rate)"
    }
    `
    
    // Parse and decode functions
    parser := hclparse.NewParser()
    file, diags := parser.ParseHCL([]byte(hclCode), "config.hcl")
    if diags.HasErrors() {
        log.Fatal(diags)
    }
    
    functions, _, diags := jqfunc.DecodeJqFunctions(file.Body, "jq")
    if diags.HasErrors() {
        log.Fatal(diags)
    }
    
    // Use the functions
    getName := functions["get_name"]
    result, _ := getName.Call([]cty.Value{
        cty.StringVal(`{"name": "Alice", "age": 30}`),
    })
    fmt.Println(result.AsString()) // Output: Alice
    
    addTax := functions["add_tax"]
    result, _ = addTax.Call([]cty.Value{
        cty.StringVal(`{"price": 100}`),
        cty.NumberFloatVal(0.08),
    })
    fmt.Println(result.AsString()) // Output: 108
}
```

### HCL Function Definition Syntax

```hcl
jq "function_name" {
    params = [param1, param2, ...]  # Parameter names (bare identifiers)
    query = "JQ_QUERY_STRING"       # JQ query with $param1, $param2, etc.
}
```

## Examples

### Data Extraction

```hcl
jq "extract_user_info" {
    params = []
    query = "{name: .name, email: .contact.email}"
}
```

```go
// JSON string input
result, _ := extractUserInfo.Call([]cty.Value{
    cty.StringVal(`{"name": "Bob", "contact": {"email": "bob@example.com"}}`),
})
// Returns: {"email":"bob@example.com","name":"Bob"}

// cty object input  
result, _ := extractUserInfo.Call([]cty.Value{
    cty.ObjectVal(map[string]cty.Value{
        "name": cty.StringVal("Bob"),
        "contact": cty.ObjectVal(map[string]cty.Value{
            "email": cty.StringVal("bob@example.com"),
        }),
    }),
})
// Returns: cty.ObjectVal with name and email fields
```

### Array Processing

```hcl
jq "filter_active_users" {
    params = []
    query = "[.users[] | select(.active == true) | .name]"
}
```

### Parameterized Queries

```hcl
jq "calculate_total" {
    params = [tax_rate, discount]
    query = ".price * (1 + $tax_rate) * (1 - $discount)"
}
```

## Documentation

### Function Definition

#### Block Syntax
- **Block Type**: Use any name (commonly `jq`)
- **Function Name**: Single label after block type
- **Parameters**: List of bare identifiers in `params` attribute
- **Query**: JQ query string in `query` attribute

#### Parameter Handling
- Parameters become JQ variables prefixed with `$`
- Example: `params = [rate, discount]` creates `$rate` and `$discount` variables
- Parameters can be any cty type and are converted to Go values for JQ processing

### Input and Output Behavior

#### Input Types
1. **JSON String**: Parsed as JSON, processed by JQ, result handling depends on output type
2. **cty Value**: Converted to Go value, processed by JQ, converted back to cty

#### Output Behavior by Input Type

| Input Type | Result Type | Output |
|------------|-------------|---------|
| JSON String | String | Direct string (no JSON encoding) |
| JSON String | Number/Object/Array | JSON-encoded string |
| cty Value | Any | Corresponding cty type |

#### Multi-Result Handling
- **Single result**: Returned directly
- **Multiple results**: Returned as array/list
- **No results**: Returns `null` (JSON string) or `cty.NullVal` (cty input)

### Error Handling

The package provides enhanced error reporting with:
- **Source Location**: Exact HCL block location for compilation errors
- **Runtime Context**: Function name and query for execution errors
- **Error Chaining**: Go 1.13+ error unwrapping support

```go
if err != nil {
    var jqErr *jqfunc.JqExecutionError
    if errors.As(err, &jqErr) {
        fmt.Printf("JQ error in %s at %s: %v\n", 
            jqErr.FunctionName, jqErr.Range, jqErr.Cause)
    }
}
```

### Advanced Features

#### Custom Block Types
```go
// Use custom block type instead of "jq"
functions, _, diags := jqfunc.DecodeJqFunctions(body, "transform")
```

#### Complex Data Transformations
```hcl
jq "process_orders" {
    params = [min_amount]
    query = """
    [.orders[] | 
     select(.total >= $min_amount) | 
     {
       id: .id,
       customer: .customer.name,
       total: .total,
       items: [.items[].name]
     }
    ]
    """
}
```

#### Conditional Logic
```hcl
jq "categorize_user" {
    params = []
    query = """
    if .age < 18 then "minor"
    elif .age < 65 then "adult"  
    else "senior"
    end
    """
}
```

### Integration with HCL

JQ functions integrate seamlessly with HCL's function system:

```hcl
# In your HCL configuration
locals {
    user_data = jsondecode(file("users.json"))
    active_users = filter_active_users(user_data)
    user_names = [for user in active_users : user.name]
}
```

### Performance Considerations

- JQ queries are compiled once during function creation
- Parameter conversion happens at runtime
- For high-frequency use, prefer cty input over JSON strings
- Complex queries may benefit from breaking into smaller functions

### Limitations

- JQ variables must be prefixed with `$` in queries
- Queries have no access to variables in the execution context, only the passed in variables.
- Queries cannot call other queries or HCL functions.
- Parameter names must be valid HCL identifiers
- JSON string input/output adds serialization overhead
- Multi-result queries with mixed types may require careful handling

## Dependencies

- `github.com/hashicorp/hcl/v2` - HCL parsing and evaluation
- `github.com/itchyny/gojq` - Pure Go JQ implementation  
- `github.com/zclconf/go-cty` - HCL type system
- `github.com/tsarna/go2cty2go` - Enhanced cty value conversion

## Acknowledgments

Special thanks to [itchyny](https://github.com/itchyny) for creating and maintaining [gojq](https://github.com/itchyny/gojq), the excellent pure Go implementation of JQ that makes this project possible. The gojq library provides the robust JQ query processing that powers all the functionality in this package.