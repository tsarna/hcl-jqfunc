package jqfunc

import (
	"testing"

	"github.com/itchyny/gojq"
)

// TestBasicSetup verifies that our dependencies are working correctly
func TestBasicSetup(t *testing.T) {
	// Test that we can parse a simple jq query
	query, err := gojq.Parse(".test")
	if err != nil {
		t.Fatalf("Failed to parse jq query: %v", err)
	}

	// Test that we can execute the query
	input := map[string]interface{}{"test": "hello world"}
	iter := query.Run(input)

	result, ok := iter.Next()
	if !ok {
		t.Fatal("Expected result from jq query")
	}

	if err, ok := result.(error); ok {
		t.Fatalf("jq query execution error: %v", err)
	}

	if result != "hello world" {
		t.Fatalf("Expected 'hello world', got %v", result)
	}
}
