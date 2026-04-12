package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/Jeffail/gabs/v2"
	"golang.org/x/term"
)

// Default columns to display per resource type when no -columns flag is specified.
// Resources not listed here fall back to generic column detection.
var defaultColumns = map[string][]string{
	"workspaces":               {"id", "name", "status", "terraform-version", "auto-apply", "execution-mode"},
	"environments":             {"id", "name", "cost-estimation-enabled", "status"},
	"runs":                     {"id", "status", "source", "is-destroy", "created-at"},
	"variables":                {"id", "key", "category", "sensitive", "final"},
	"tags":                     {"id", "name"},
	"accounts":                 {"id", "name"},
	"policy-groups":            {"id", "name", "status", "opa-version"},
	"provider-configurations":  {"id", "name", "provider-name", "export-shell-variables"},
	"service-accounts":         {"id", "email", "status", "description"},
	"roles":                    {"id", "name", "is-system"},
	"teams":                    {"id", "name"},
	"users":                    {"id", "email", "status", "username"},
	"vcs-providers":            {"id", "name", "vcs-type", "url"},
	"access-policies":          {"id", "is-system"},
	"agent-pools":              {"id", "name", "vcs-enabled"},
	"modules":                  {"id", "name", "provider", "status"},
	"webhooks":                 {"id", "name", "enabled", "url"},
	"access-tokens":            {"id", "description", "created-at"},
	"configuration-versions":   {"id", "status", "source"},
	"applies":                  {"id", "status"},
	"plans":                    {"id", "status", "has-changes"},
	"state-versions":           {"id", "serial", "created-at"},
	"policy-checks":            {"id", "status", "scope"},
}

// isTerminal returns true if stdout is connected to a terminal (not piped)
func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// resolveFormat determines the actual output format.
// JSON is always the default to avoid breaking existing scripts.
// Table/CSV output requires an explicit -format=table or -format=csv flag.
func resolveFormat(format string) string {
	if format != "" {
		return strings.ToLower(strings.TrimSpace(format))
	}
	return "json"
}

// formatOutput dispatches to the appropriate formatter based on the format string.
// data is the parsed JSONAPI output (either a single object or array).
// isArray indicates whether the response is a list of items.
// columns is the user-specified column list (empty means auto-detect).
// resourceType is used to look up default columns.
func formatOutput(data *gabs.Container, format string, isArray bool, columns string, resourceType string) {
	switch format {
	case "table":
		if isArray {
			formatTable(data, columns, resourceType)
		} else {
			formatKeyValue(data)
		}
	case "csv":
		formatCSV(data, columns, resourceType, isArray)
	default:
		// JSON (default)
		fmt.Println(data.StringIndent("", "  "))
	}
}

// formatTable renders a list of items as an aligned table.
func formatTable(data *gabs.Container, columns string, resourceType string) {
	children := data.Children()
	if len(children) == 0 {
		fmt.Fprintln(os.Stderr, "No results found.")
		return
	}

	cols := resolveColumns(children[0], columns, resourceType)
	if len(cols) == 0 {
		// Last resort: show all keys from the first item, no filtering at all
		for k := range children[0].ChildrenMap() {
			cols = append(cols, k)
		}
		sort.Strings(cols)
	}
	if len(cols) == 0 {
		// Truly empty object — nothing to display
		fmt.Fprintln(os.Stderr, "No results found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	// Header
	headers := make([]string, len(cols))
	for i, col := range cols {
		headers[i] = strings.ToUpper(col)
	}
	fmt.Fprintln(w, strings.Join(headers, "\t"))

	// Separator
	seps := make([]string, len(cols))
	for i, h := range headers {
		seps[i] = strings.Repeat("-", len(h))
	}
	fmt.Fprintln(w, strings.Join(seps, "\t"))

	// Rows
	for _, item := range children {
		vals := make([]string, len(cols))
		for i, col := range cols {
			vals[i] = extractValue(item, col)
		}
		fmt.Fprintln(w, strings.Join(vals, "\t"))
	}

	w.Flush()
}

// formatKeyValue renders a single object as key-value pairs (like kubectl describe).
func formatKeyValue(data *gabs.Container) {
	flat := data.ChildrenMap()
	if len(flat) == 0 {
		// This should not happen for successful API responses — the empty-result
		// check in callAPI should catch it first. If we get here, print the raw
		// container so the user at least sees something instead of a confusing message.
		raw := data.StringIndent("", "  ")
		if raw != "{}" && raw != "null" && raw != "" {
			fmt.Println(raw)
		}
		return
	}

	// Sort keys for consistent output
	keys := make([]string, 0, len(flat))
	maxLen := 0
	for k := range flat {
		keys = append(keys, k)
		if len(k) > maxLen {
			maxLen = len(k)
		}
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := flat[k]
		val := formatScalar(v)
		fmt.Printf("%-*s  %s\n", maxLen, strings.ToUpper(k)+":", val)
	}
}

// formatCSV renders data as RFC 4180 CSV.
func formatCSV(data *gabs.Container, columns string, resourceType string, isArray bool) {
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

	if !isArray {
		// Single object: output as two-column CSV (key, value)
		flat := data.ChildrenMap()
		keys := make([]string, 0, len(flat))
		for k := range flat {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		w.Write([]string{"field", "value"})
		for _, k := range keys {
			w.Write([]string{k, sanitizeCSV(formatScalar(flat[k]))})
		}
		return
	}

	children := data.Children()
	if len(children) == 0 {
		return
	}

	cols := resolveColumns(children[0], columns, resourceType)
	if len(cols) == 0 {
		return
	}

	// Header
	w.Write(cols)

	// Rows
	for _, item := range children {
		row := make([]string, len(cols))
		for i, col := range cols {
			row[i] = sanitizeCSV(extractValue(item, col))
		}
		w.Write(row)
	}
}

// resolveColumns determines which columns to display.
// Priority: explicit -columns flag > resource-type defaults > auto-detect from first item.
func resolveColumns(firstItem *gabs.Container, columns string, resourceType string) []string {
	if columns != "" {
		return strings.Split(columns, ",")
	}

	// Try to get resource type from the item's "type" field (JSONAPI standard, always plural)
	// This is more reliable than the x-resource extension which may be singular/PascalCase.
	if firstItem.Exists("type") {
		if typeVal, ok := firstItem.Path("type").Data().(string); ok && typeVal != "" {
			resourceType = typeVal
		} else if typeContainer, ok := firstItem.Path("type").Data().(*gabs.Container); ok {
			if s, ok := typeContainer.Data().(string); ok && s != "" {
				resourceType = s
			}
		}
	}

	if resourceType != "" {
		if cols, ok := defaultColumns[resourceType]; ok {
			// Filter to only columns that actually exist in the data
			existing := make([]string, 0, len(cols))
			for _, col := range cols {
				if firstItem.Exists(col) {
					existing = append(existing, col)
				}
			}
			if len(existing) > 0 {
				return existing
			}
		}
	}

	// Auto-detect: use id, name, type, status, then first N keys
	return autoDetectColumns(firstItem)
}

// autoDetectColumns picks a reasonable set of columns from the first item.
// Always returns at least something — falls back to all keys if filtering is too aggressive.
func autoDetectColumns(item *gabs.Container) []string {
	preferred := []string{"id", "name", "type", "status", "key", "email"}
	flat := item.ChildrenMap()

	cols := make([]string, 0, 6)
	for _, p := range preferred {
		if _, ok := flat[p]; ok {
			cols = append(cols, p)
		}
	}

	// Collect remaining scalar keys (skip nested objects/arrays)
	remaining := make([]string, 0)
	for k, v := range flat {
		if containsString(cols, k) {
			continue
		}
		// Skip nested objects/arrays — they don't render well in tables
		if _, isMap := v.Data().(map[string]interface{}); isMap {
			continue
		}
		if _, isArr := v.Data().([]interface{}); isArr {
			continue
		}
		remaining = append(remaining, k)
	}
	sort.Strings(remaining)
	for _, r := range remaining {
		if len(cols) >= 6 {
			break
		}
		cols = append(cols, r)
	}

	// Fallback: if aggressive filtering removed everything, include ALL keys
	// (even nested ones) so the user sees something rather than "No displayable columns"
	if len(cols) == 0 {
		allKeys := make([]string, 0, len(flat))
		for k := range flat {
			allKeys = append(allKeys, k)
		}
		sort.Strings(allKeys)
		if len(allKeys) > 6 {
			allKeys = allKeys[:6]
		}
		return allKeys
	}

	return cols
}

// extractValue safely extracts a display-friendly string from a gabs container field.
func extractValue(item *gabs.Container, field string) string {
	v := item.Path(field)
	if v == nil || v.Data() == nil {
		return ""
	}
	return formatScalar(v)
}

// formatScalar converts a gabs value to a display string.
// Handles *gabs.Container wrappers that parseData creates for id/type fields.
func formatScalar(v *gabs.Container) string {
	if v == nil || v.Data() == nil {
		return ""
	}

	// Unwrap nested *gabs.Container (parseData uses SetP with containers)
	data := v.Data()
	if inner, ok := data.(*gabs.Container); ok {
		if inner == nil || inner.Data() == nil {
			return ""
		}
		data = inner.Data()
	}

	switch val := data.(type) {
	case string:
		return val
	case bool:
		if val {
			return "true"
		}
		return "false"
	case float64:
		if val == float64(int64(val)) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case map[string]interface{}:
		// For nested objects, show the ID if available, otherwise compact JSON
		if id, ok := val["id"]; ok {
			return fmt.Sprintf("%v", id)
		}
		return v.String()
	case []interface{}:
		if len(val) == 0 {
			return "[]"
		}
		// For arrays of simple values, join with commas
		parts := make([]string, 0, len(val))
		for _, item := range val {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprintf("%v", val)
	}
}

// sanitizeCSV prevents spreadsheet formula injection by prefixing dangerous
// starting characters with a single quote. Spreadsheet programs interpret
// cells starting with =, +, -, @ as formulas.
func sanitizeCSV(val string) string {
	if len(val) > 0 {
		switch val[0] {
		case '=', '+', '-', '@':
			return "'" + val
		}
	}
	return val
}

// containsString checks if a string slice contains a value.
func containsString(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

// filterFields filters a gabs container to include only the specified fields.
// Works for both single objects and arrays.
func filterFields(data *gabs.Container, fields string, isArray bool) *gabs.Container {
	if fields == "" {
		return data
	}

	fieldList := strings.Split(fields, ",")

	if !isArray {
		return filterSingleObject(data, fieldList)
	}

	result := gabs.New()
	result.Array()
	for _, child := range data.Children() {
		result.ArrayAppend(filterSingleObject(child, fieldList).Data())
	}
	return result
}

// filterSingleObject creates a new container with only the specified keys.
func filterSingleObject(item *gabs.Container, fields []string) *gabs.Container {
	result := gabs.New()
	for _, f := range fields {
		f = strings.TrimSpace(f)
		if item.Exists(f) {
			result.Set(item.Path(f).Data(), f)
		}
	}
	return result
}

// formatPaginationInfo prints pagination metadata to stderr (only in table mode).
func formatPaginationInfo(currentPage int, totalPages interface{}, totalCount interface{}) {
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return
	}

	parts := make([]string, 0, 2)
	if totalPages != nil {
		parts = append(parts, fmt.Sprintf("page %d of %v", currentPage, totalPages))
	}
	if totalCount != nil {
		parts = append(parts, fmt.Sprintf("%v total", totalCount))
	}
	if len(parts) > 0 {
		fmt.Fprintf(os.Stderr, "(%s)\n", strings.Join(parts, ", "))
	}
}
