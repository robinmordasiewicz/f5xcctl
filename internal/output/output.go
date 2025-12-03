// Package output provides output formatting for the f5xcctl CLI.
package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/tabwriter"
	"text/template"

	"gopkg.in/yaml.v3"
)

// Formatter formats output data.
type Formatter interface {
	// Format formats the data and writes to the writer
	Format(w io.Writer, data interface{}) error
}

// NewFormatter creates a formatter for the specified format.
func NewFormatter(format string) Formatter {
	format = strings.ToLower(format)

	// Handle parameterized formats
	if strings.HasPrefix(format, "jsonpath=") {
		expr := strings.TrimPrefix(format, "jsonpath=")
		return &JSONPathFormatter{Expression: expr}
	}
	if strings.HasPrefix(format, "custom-columns=") {
		spec := strings.TrimPrefix(format, "custom-columns=")
		return &CustomColumnsFormatter{Spec: spec}
	}
	if strings.HasPrefix(format, "go-template=") {
		tmpl := strings.TrimPrefix(format, "go-template=")
		return &GoTemplateFormatter{Template: tmpl}
	}

	switch format {
	case "json":
		return &JSONFormatter{Indent: true}
	case "yaml":
		return &YAMLFormatter{}
	case "text":
		return &TextFormatter{}
	case "jsonpath":
		return &JSONPathFormatter{Expression: ""}
	case "table":
		fallthrough
	default:
		return &TableFormatter{}
	}
}

// Print formats and prints data to stdout.
func Print(format string, data interface{}) error {
	formatter := NewFormatter(format)
	return formatter.Format(os.Stdout, data)
}

// JSONFormatter formats output as JSON.
type JSONFormatter struct {
	Indent bool
}

// Format formats data as JSON.
func (f *JSONFormatter) Format(w io.Writer, data interface{}) error {
	encoder := json.NewEncoder(w)
	if f.Indent {
		encoder.SetIndent("", "  ")
	}
	return encoder.Encode(data)
}

// YAMLFormatter formats output as YAML.
type YAMLFormatter struct{}

// Format formats data as YAML.
func (f *YAMLFormatter) Format(w io.Writer, data interface{}) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)
	return encoder.Encode(data)
}

// TextFormatter formats output as plain text.
type TextFormatter struct{}

// Format formats data as plain text.
func (f *TextFormatter) Format(w io.Writer, data interface{}) error {
	switch v := data.(type) {
	case string:
		fmt.Fprintln(w, v)
	case []byte:
		fmt.Fprintln(w, string(v))
	case fmt.Stringer:
		fmt.Fprintln(w, v.String())
	default:
		// For complex types, just use JSON
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(data)
	}
	return nil
}

// TableFormatter formats output as a table using text/tabwriter.
type TableFormatter struct {
	// Headers to use (if empty, derived from data)
	Headers []string
	// Keys to extract from data (if empty, all keys)
	Keys []string
}

// Format formats data as a table.
func (f *TableFormatter) Format(w io.Writer, data interface{}) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer tw.Flush()

	// Handle different data types
	val := reflect.ValueOf(data)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Slice, reflect.Array:
		return f.formatSlice(tw, val)
	case reflect.Struct:
		return f.formatStruct(tw, val)
	case reflect.Map:
		return f.formatMap(tw, val)
	default:
		// For simple types, just print
		fmt.Fprintln(w, data)
		return nil
	}
}

func (f *TableFormatter) formatSlice(tw *tabwriter.Writer, val reflect.Value) error {
	if val.Len() == 0 {
		fmt.Println("No resources found")
		return nil
	}

	// Get headers from first element
	first := val.Index(0)
	if first.Kind() == reflect.Ptr {
		first = first.Elem()
	}

	headers, keys := f.getHeadersAndKeys(first)

	// Print header
	fmt.Fprintln(tw, strings.Join(headers, "\t"))

	// Print rows
	for i := 0; i < val.Len(); i++ {
		elem := val.Index(i)
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		row := f.extractRow(elem, keys)
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}

	return nil
}

func (f *TableFormatter) formatStruct(tw *tabwriter.Writer, val reflect.Value) error {
	headers, keys := f.getHeadersAndKeys(val)

	// Print header
	fmt.Fprintln(tw, strings.Join(headers, "\t"))

	// Print row
	row := f.extractRow(val, keys)
	fmt.Fprintln(tw, strings.Join(row, "\t"))

	return nil
}

func (f *TableFormatter) formatMap(tw *tabwriter.Writer, val reflect.Value) error {
	fmt.Fprintln(tw, "KEY\tVALUE")

	for _, key := range val.MapKeys() {
		value := val.MapIndex(key)
		fmt.Fprintf(tw, "%v\t%v\n", key.Interface(), value.Interface())
	}

	return nil
}

func (f *TableFormatter) getHeadersAndKeys(val reflect.Value) ([]string, []string) {
	if len(f.Headers) > 0 && len(f.Keys) > 0 {
		return f.Headers, f.Keys
	}

	var headers, keys []string

	switch val.Kind() {
	case reflect.Struct:
		typ := val.Type()
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.PkgPath != "" { // Skip unexported fields
				continue
			}

			// Get JSON tag or use field name
			name := field.Name
			if tag := field.Tag.Get("json"); tag != "" {
				parts := strings.Split(tag, ",")
				if parts[0] != "-" {
					name = parts[0]
				}
			}

			headers = append(headers, strings.ToUpper(name))
			keys = append(keys, field.Name)
		}
	case reflect.Map:
		headers = []string{"KEY", "VALUE"}
		keys = []string{"key", "value"}
	}

	return headers, keys
}

func (f *TableFormatter) extractRow(val reflect.Value, keys []string) []string {
	var row []string

	switch val.Kind() {
	case reflect.Struct:
		for _, key := range keys {
			field := val.FieldByName(key)
			if !field.IsValid() {
				row = append(row, "")
				continue
			}
			row = append(row, formatValue(field))
		}
	case reflect.Map:
		for _, key := range keys {
			v := val.MapIndex(reflect.ValueOf(key))
			if !v.IsValid() {
				row = append(row, "")
				continue
			}
			row = append(row, formatValue(v))
		}
	}

	return row
}

func formatValue(val reflect.Value) string {
	if !val.IsValid() {
		return ""
	}

	// Handle nil pointers
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return ""
		}
		val = val.Elem()
	}

	// Handle nil interfaces
	if val.Kind() == reflect.Interface {
		if val.IsNil() {
			return ""
		}
		val = val.Elem()
	}

	switch val.Kind() {
	case reflect.Slice, reflect.Array:
		if val.Len() == 0 {
			return ""
		}
		// Show count for arrays
		return fmt.Sprintf("[%d items]", val.Len())
	case reflect.Map:
		if val.Len() == 0 {
			return ""
		}
		return fmt.Sprintf("{%d keys}", val.Len())
	case reflect.Struct:
		// Try to get a name or string representation
		if nameField := val.FieldByName("Name"); nameField.IsValid() {
			return formatValue(nameField)
		}
		return "{...}"
	default:
		return fmt.Sprintf("%v", val.Interface())
	}
}

// Resource represents a generic resource for table output.
type Resource struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	Status    string `json:"status,omitempty"`
	Age       string `json:"age,omitempty"`
}

// ResourceList is a list of resources.
type ResourceList struct {
	Items []Resource `json:"items"`
}

// NamespaceOutput represents namespace data for output.
type NamespaceOutput struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status"`
}

// Error prints an error message to stderr.
func Error(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
}

// Success prints a success message.
func Success(format string, args ...interface{}) {
	fmt.Printf("✓ "+format+"\n", args...)
}

// Info prints an info message.
func Info(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

// Warning prints a warning message.
func Warning(format string, args ...interface{}) {
	fmt.Printf("Warning: "+format+"\n", args...)
}

// ============================================================================
// JSONPath Formatter
// ============================================================================

// JSONPathFormatter formats output using JSONPath expressions.
type JSONPathFormatter struct {
	Expression string
}

// Format formats data using JSONPath expression.
func (f *JSONPathFormatter) Format(w io.Writer, data interface{}) error {
	if f.Expression == "" {
		return fmt.Errorf("jsonpath expression is required")
	}

	// Clean expression
	expr := strings.Trim(f.Expression, "'\"")
	expr = strings.TrimPrefix(expr, "{")
	expr = strings.TrimSuffix(expr, "}")

	// Convert data to map for easier traversal
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	var dataMap interface{}
	if err := json.Unmarshal(jsonData, &dataMap); err != nil {
		return fmt.Errorf("failed to unmarshal data: %w", err)
	}

	result := evaluateJSONPath(dataMap, expr)

	// Format output
	switch v := result.(type) {
	case []interface{}:
		for _, item := range v {
			fmt.Fprintln(w, formatJSONPathValue(item))
		}
	case nil:
		// No output for nil
	default:
		fmt.Fprintln(w, formatJSONPathValue(v))
	}

	return nil
}

// evaluateJSONPath evaluates a JSONPath-like expression
// Supports: .field, .field.subfield, .items[*].name, .items[0].
func evaluateJSONPath(data interface{}, expr string) interface{} {
	if expr == "" || expr == "." {
		return data
	}

	// Remove leading dot
	expr = strings.TrimPrefix(expr, ".")

	// Split by dots (handling array notation)
	parts := splitJSONPathParts(expr)

	current := data
	for _, part := range parts {
		if current == nil {
			return nil
		}

		// Check for array indexing
		if strings.Contains(part, "[") {
			arrayPart, indexStr := parseArrayPart(part)

			// Get the array first
			if arrayPart != "" {
				current = getField(current, arrayPart)
				if current == nil {
					return nil
				}
			}

			// Handle array index
			arr, ok := current.([]interface{})
			if !ok {
				return nil
			}

			if indexStr == "*" {
				// Wildcard - return all elements
				// Process remaining path for each element
				return current
			}

			idx, err := strconv.Atoi(indexStr)
			if err != nil {
				return nil
			}

			if idx < 0 || idx >= len(arr) {
				return nil
			}
			current = arr[idx]
		} else {
			current = getField(current, part)
		}
	}

	// Handle wildcard expansion
	if arr, ok := current.([]interface{}); ok {
		// Check if we need to extract a field from each element
		remaining := findRemainingPath(expr)
		if remaining != "" {
			var results []interface{}
			for _, item := range arr {
				result := evaluateJSONPath(item, remaining)
				if result != nil {
					results = append(results, result)
				}
			}
			return results
		}
	}

	return current
}

// splitJSONPathParts splits a JSONPath expression into parts.
func splitJSONPathParts(expr string) []string {
	var parts []string
	var current strings.Builder
	bracketDepth := 0

	for _, ch := range expr {
		switch ch {
		case '[':
			bracketDepth++
			current.WriteRune(ch)
		case ']':
			bracketDepth--
			current.WriteRune(ch)
		case '.':
			if bracketDepth == 0 {
				if current.Len() > 0 {
					parts = append(parts, current.String())
					current.Reset()
				}
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// parseArrayPart parses "items[0]" into ("items", "0").
func parseArrayPart(part string) (string, string) {
	idx := strings.Index(part, "[")
	if idx == -1 {
		return part, ""
	}

	arrayName := part[:idx]
	indexStr := strings.Trim(part[idx:], "[]")
	return arrayName, indexStr
}

// getField gets a field from a map or struct.
func getField(data interface{}, field string) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		return v[field]
	case map[interface{}]interface{}:
		return v[field]
	default:
		// Use reflection for structs
		val := reflect.ValueOf(data)
		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		if val.Kind() == reflect.Struct {
			f := val.FieldByName(field)
			if f.IsValid() {
				return f.Interface()
			}
		}
		return nil
	}
}

// findRemainingPath finds the path after [*].
func findRemainingPath(expr string) string {
	idx := strings.Index(expr, "[*]")
	if idx == -1 {
		return ""
	}
	remaining := expr[idx+3:]
	return strings.TrimPrefix(remaining, ".")
}

// formatJSONPathValue formats a value for JSONPath output.
func formatJSONPathValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", val)
	}
}

// ============================================================================
// Custom Columns Formatter
// ============================================================================

// CustomColumnsFormatter formats output using custom column specifications.
type CustomColumnsFormatter struct {
	Spec string // "NAME:.metadata.name,NAMESPACE:.metadata.namespace"
}

// ColumnSpec represents a custom column specification.
type ColumnSpec struct {
	Header   string
	JSONPath string
}

// Format formats data using custom columns.
func (f *CustomColumnsFormatter) Format(w io.Writer, data interface{}) error {
	if f.Spec == "" {
		return fmt.Errorf("custom-columns specification is required")
	}

	columns := parseColumnSpec(f.Spec)
	if len(columns) == 0 {
		return fmt.Errorf("no columns specified")
	}

	// Convert data to JSON for processing
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	var dataMap map[string]interface{}
	if err := json.Unmarshal(jsonData, &dataMap); err != nil {
		return fmt.Errorf("failed to unmarshal data: %w", err)
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer tw.Flush()

	// Print headers
	headers := make([]string, len(columns))
	for i, col := range columns {
		headers[i] = col.Header
	}
	fmt.Fprintln(tw, strings.Join(headers, "\t"))

	// Check if data has items (list) or is a single resource
	items, ok := dataMap["items"].([]interface{})
	if !ok {
		// Single resource
		items = []interface{}{dataMap}
	}

	// Print rows
	for _, item := range items {
		row := make([]string, len(columns))
		for i, col := range columns {
			value := evaluateJSONPath(item, col.JSONPath)
			row[i] = formatJSONPathValue(value)
		}
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}

	return nil
}

// parseColumnSpec parses a custom column specification string
// Format: "NAME:.metadata.name,NAMESPACE:.metadata.namespace".
func parseColumnSpec(spec string) []ColumnSpec {
	var columns []ColumnSpec

	parts := strings.Split(spec, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		colonIdx := strings.Index(part, ":")
		if colonIdx == -1 {
			continue
		}

		header := strings.TrimSpace(part[:colonIdx])
		jsonPath := strings.TrimSpace(part[colonIdx+1:])

		if header != "" && jsonPath != "" {
			columns = append(columns, ColumnSpec{
				Header:   header,
				JSONPath: jsonPath,
			})
		}
	}

	return columns
}

// ============================================================================
// Go Template Formatter
// ============================================================================

// GoTemplateFormatter formats output using Go templates.
type GoTemplateFormatter struct {
	Template string
}

// Format formats data using a Go template.
func (f *GoTemplateFormatter) Format(w io.Writer, data interface{}) error {
	if f.Template == "" {
		return fmt.Errorf("go-template is required")
	}

	// Convert data to map for template processing
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	var dataMap interface{}
	if err := json.Unmarshal(jsonData, &dataMap); err != nil {
		return fmt.Errorf("failed to unmarshal data: %w", err)
	}

	// Parse and execute template
	tmpl, err := template.New("output").Funcs(templateFuncs()).Parse(f.Template)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, dataMap); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	fmt.Fprintln(w, buf.String())
	return nil
}

// templateFuncs returns helper functions for Go templates.
func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"json": func(v interface{}) string {
			data, _ := json.Marshal(v)
			return string(data)
		},
		"yaml": func(v interface{}) string {
			data, _ := yaml.Marshal(v)
			return string(data)
		},
		"join":  strings.Join,
		"split": strings.Split,
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"trim":  strings.TrimSpace,
		"index": func(v interface{}, i int) interface{} {
			switch arr := v.(type) {
			case []interface{}:
				if i >= 0 && i < len(arr) {
					return arr[i]
				}
			case []string:
				if i >= 0 && i < len(arr) {
					return arr[i]
				}
			}
			return nil
		},
		"len": func(v interface{}) int {
			switch arr := v.(type) {
			case []interface{}:
				return len(arr)
			case []string:
				return len(arr)
			case map[string]interface{}:
				return len(arr)
			case string:
				return len(arr)
			}
			return 0
		},
		"default": func(def interface{}, v interface{}) interface{} {
			if v == nil {
				return def
			}
			if s, ok := v.(string); ok && s == "" {
				return def
			}
			return v
		},
	}
}
