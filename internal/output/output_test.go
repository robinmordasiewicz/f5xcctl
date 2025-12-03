package output

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type testResource struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Count  int    `json:"count"`
}

func TestJSONFormatter(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		indent   bool
		expected string
	}{
		{
			name: "simple struct",
			data: testResource{Name: "test", Status: "active", Count: 5},
		},
		{
			name: "slice of structs",
			data: []testResource{
				{Name: "test1", Status: "active", Count: 1},
				{Name: "test2", Status: "inactive", Count: 2},
			},
		},
		{
			name: "map",
			data: map[string]string{"key": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			formatter := &JSONFormatter{Indent: true}

			err := formatter.Format(&buf, tt.data)
			require.NoError(t, err)

			// Verify valid JSON
			var decoded interface{}
			err = json.Unmarshal(buf.Bytes(), &decoded)
			assert.NoError(t, err)
		})
	}
}

func TestYAMLFormatter(t *testing.T) {
	tests := []struct {
		name string
		data interface{}
	}{
		{
			name: "simple struct",
			data: testResource{Name: "test", Status: "active", Count: 5},
		},
		{
			name: "slice of structs",
			data: []testResource{
				{Name: "test1", Status: "active", Count: 1},
				{Name: "test2", Status: "inactive", Count: 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			formatter := &YAMLFormatter{}

			err := formatter.Format(&buf, tt.data)
			require.NoError(t, err)

			// Verify valid YAML
			var decoded interface{}
			err = yaml.Unmarshal(buf.Bytes(), &decoded)
			assert.NoError(t, err)
		})
	}
}

func TestTableFormatter(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		contains []string
	}{
		{
			name:     "single struct",
			data:     testResource{Name: "test", Status: "active", Count: 5},
			contains: []string{"NAME", "STATUS", "COUNT", "test", "active", "5"},
		},
		{
			name: "slice of structs",
			data: []testResource{
				{Name: "test1", Status: "active", Count: 1},
				{Name: "test2", Status: "inactive", Count: 2},
			},
			contains: []string{"NAME", "test1", "test2", "active", "inactive"},
		},
		{
			name:     "map",
			data:     map[string]string{"key1": "value1", "key2": "value2"},
			contains: []string{"KEY", "VALUE"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			formatter := &TableFormatter{}

			err := formatter.Format(&buf, tt.data)
			require.NoError(t, err)

			output := buf.String()
			for _, s := range tt.contains {
				assert.Contains(t, output, s)
			}
		})
	}
}

func TestTableFormatterEmptySlice(t *testing.T) {
	var buf bytes.Buffer
	formatter := &TableFormatter{}

	err := formatter.Format(&buf, []testResource{})
	require.NoError(t, err)
	// Should output "No resources found"
}

func TestNewFormatter(t *testing.T) {
	tests := []struct {
		format       string
		expectedType string
	}{
		{"json", "*output.JSONFormatter"},
		{"JSON", "*output.JSONFormatter"},
		{"yaml", "*output.YAMLFormatter"},
		{"YAML", "*output.YAMLFormatter"},
		{"text", "*output.TextFormatter"},
		{"table", "*output.TableFormatter"},
		{"", "*output.TableFormatter"},
		{"unknown", "*output.TableFormatter"},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			formatter := NewFormatter(tt.format)
			assert.NotNil(t, formatter)
		})
	}
}

func TestTextFormatter(t *testing.T) {
	tests := []struct {
		name     string
		data     interface{}
		expected string
	}{
		{
			name:     "string",
			data:     "hello world",
			expected: "hello world\n",
		},
		{
			name:     "bytes",
			data:     []byte("hello bytes"),
			expected: "hello bytes\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			formatter := &TextFormatter{}

			err := formatter.Format(&buf, tt.data)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, buf.String())
		})
	}
}

func TestFormatValueNilHandling(t *testing.T) {
	// Test nil pointer
	var nilPtr *string
	result := formatValue(reflect.ValueOf(nilPtr))
	assert.Equal(t, "", result)

	// Test valid pointer
	str := "test"
	result = formatValue(reflect.ValueOf(&str))
	assert.Equal(t, "test", result)
}
