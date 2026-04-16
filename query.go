package main

import (
	"fmt"
	"strings"

	"github.com/Jeffail/gabs/v2"
)

// applyQuery applies a simple dot-path expression to extract data from a gabs container.
//
// Supported syntax:
//   - ".field"           — extract a single field from an object
//   - ".[].field"        — extract a field from each element of an array
//   - ".field1.field2"   — nested field access
//   - ".[].field1.field2" — nested access within each array element
//
// Returns the filtered container and a flag indicating if the result is a simple value
// (which should be printed as plain text, not JSON).
func applyQuery(data *gabs.Container, query string, isArray bool) (*gabs.Container, bool) {
	if query == "" {
		return data, false
	}

	// Remove leading dot if present
	query = strings.TrimPrefix(query, ".")

	// Handle array iteration: .[].field
	if strings.HasPrefix(query, "[].") {
		path := strings.TrimPrefix(query, "[].")
		return extractFromArray(data, path)
	}

	// Handle array iteration with just .[] (return all items)
	if query == "[]" {
		return data, false
	}

	// Simple field extraction from object or array
	if isArray {
		// If the data is an array but user asked for .field, treat it as .[].field
		return extractFromArray(data, query)
	}

	// Extract from single object
	result := data.Path(query)
	if result == nil || result.Data() == nil {
		return gabs.New(), true
	}

	return result, isSimpleValue(result)
}

// extractFromArray extracts a field path from each element of an array.
func extractFromArray(data *gabs.Container, path string) (*gabs.Container, bool) {
	children := data.Children()
	if len(children) == 0 {
		result := gabs.New()
		result.Array()
		return result, false
	}

	result := gabs.New()
	result.Array()

	allSimple := true
	for _, child := range children {
		val := child.Path(path)
		if val != nil && val.Data() != nil {
			result.ArrayAppend(val.Data())
			if !isSimpleValue(val) {
				allSimple = false
			}
		}
	}

	// If all values are simple scalars, print them one per line
	if allSimple {
		return result, true
	}

	return result, false
}

// isSimpleValue checks if a gabs container holds a simple scalar (string, number, bool).
func isSimpleValue(v *gabs.Container) bool {
	switch v.Data().(type) {
	case string, float64, bool:
		return true
	default:
		return false
	}
}

// formatQueryResult prints query results. Simple values are printed as plain text (one per line).
func formatQueryResult(data *gabs.Container, isSimple bool) {
	if isSimple {
		// Print simple values as plain text
		if children := data.Children(); len(children) > 0 {
			// Array of simple values — one per line
			for _, child := range children {
				fmt.Println(formatScalar(child))
			}
		} else {
			// Single simple value
			fmt.Println(formatScalar(data))
		}
		return
	}

	// Complex values: print as JSON
	fmt.Println(data.StringIndent("", "  "))
}
