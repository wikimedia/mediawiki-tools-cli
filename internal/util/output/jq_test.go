package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrintJQ_StringField(t *testing.T) {
	objects := map[string]interface{}{
		"Version": "1.2.3",
	}
	var buf bytes.Buffer
	printJQ(objects, ".Version", &buf)
	assert.Equal(t, "1.2.3\n", buf.String())
}

func TestPrintJQ_NestedField(t *testing.T) {
	objects := map[string]interface{}{
		"info": map[string]interface{}{
			"name": "mw",
		},
	}
	var buf bytes.Buffer
	printJQ(objects, ".info.name", &buf)
	assert.Equal(t, "mw\n", buf.String())
}

func TestPrintJQ_ArrayIteration(t *testing.T) {
	objects := map[interface{}]interface{}{
		"a": map[interface{}]interface{}{"Name": "alpha"},
		"b": map[interface{}]interface{}{"Name": "beta"},
	}
	var buf bytes.Buffer
	printJQ(objects, ".[] | .Name", &buf)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	assert.ElementsMatch(t, []string{"alpha", "beta"}, lines)
}

func TestPrintJQ_InterfaceKeyMap(t *testing.T) {
	objects := map[interface{}]interface{}{
		"Version": "2.0.0",
	}
	var buf bytes.Buffer
	printJQ(objects, ".Version", &buf)
	assert.Equal(t, "2.0.0\n", buf.String())
}

func TestPrintJQ_EmptyFormat_FallsBackToJSON(t *testing.T) {
	objects := map[string]interface{}{
		"key": "val",
	}
	var buf bytes.Buffer
	// Empty format should produce JSON output without panicking.
	printJQ(objects, "", &buf)
	assert.Contains(t, buf.String(), `"key"`)
	assert.Contains(t, buf.String(), `"val"`)
}

func TestPrintJQ_InvalidQuery(t *testing.T) {
	var buf bytes.Buffer
	// Should log error but not panic.
	printJQ(map[string]interface{}{}, "!!invalid!!", &buf)
	assert.Empty(t, buf.String())
}

func TestNormalizeInput_InterfaceKeyMap(t *testing.T) {
	in := map[interface{}]interface{}{
		"key": "value",
		42:    "num-key",
	}
	out := normalizeInput(in)
	m, ok := out.(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "value", m["key"])
	assert.Equal(t, "num-key", m["42"])
}

func TestNormalizeInput_NestedSlice(t *testing.T) {
	in := []interface{}{
		map[interface{}]interface{}{"x": 1},
	}
	out := normalizeInput(in)
	sl, ok := out.([]interface{})
	assert.True(t, ok)
	m, ok := sl[0].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, 1, m["x"])
}
