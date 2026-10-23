package output

import (
	"bytes"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mattn/go-runewidth"
)

func TestGraphBarsFitTerminalAndRetainRankOrder(t *testing.T) {
	for _, width := range []int{24, 40, 80} {
		var buf bytes.Buffer
		err := (Graph{Title: "Most viewed pages", Bars: true, Width: width, Points: []GraphPoint{
			{Label: "First", Value: 100},
			{Label: "Second", Value: 50},
			{Label: "日本語の長い記事名", Value: 0},
			{Label: "Control\n\x1bLabel", Value: 1},
		}}).Print(&buf)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
		if !strings.HasPrefix(lines[1], "First") || !strings.HasPrefix(lines[2], "Second") {
			t.Fatalf("ranking changed: %s", buf.String())
		}
		full, half := strings.Count(lines[1], "█"), strings.Count(lines[2], "█")
		if full == 0 || math.Abs(float64(full)/2-float64(half)) > 0.5 {
			t.Fatalf("bar proportions: %s", buf.String())
		}
		if strings.Contains(lines[3], "█") || strings.Contains(buf.String(), "\x1b") {
			t.Fatalf("zero bar or unsafe label: %s", buf.String())
		}
		assertGraphWidth(t, buf.String(), width)
	}
}

func TestGraphUsesDatesForSpacingAndDoesNotMutatePoints(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	points := []GraphPoint{{Time: start.AddDate(0, 0, 10), Value: 10}, {Time: start, Value: 0}, {Time: start.AddDate(0, 0, 1), Value: 5}}
	original := append([]GraphPoint(nil), points...)
	var buf bytes.Buffer
	if err := (Graph{Points: points, Width: 60}).Print(&buf); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(points, original) {
		t.Fatal("rendering changed input order")
	}
	var positions []int
	for _, line := range strings.Split(buf.String(), "\n") {
		_, plot, ok := strings.Cut(line, "┤")
		if !ok {
			continue
		}
		for x, char := range []rune(plot) {
			if char == '●' {
				positions = append(positions, x)
			}
		}
	}
	if len(positions) != 3 || positions[0] < 45 || positions[1] < 3 || positions[1] > 8 || positions[2] != 0 {
		t.Fatalf("dates not spaced proportionately: %v\n%s", positions, buf.String())
	}
	if !strings.Contains(buf.String(), "2025-01-01") || !strings.Contains(buf.String(), "2025-01-11") {
		t.Fatal("missing date labels")
	}
	assertGraphWidth(t, buf.String(), 60)
}

func TestGraphEmptyConstantAndDenseSeries(t *testing.T) {
	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, value := range []float64{0, 42, -42, 1000000001} {
		var buf bytes.Buffer
		if err := (Graph{Width: 40, Points: []GraphPoint{{Time: start, Value: value}}}).Print(&buf); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(buf.String(), "●") {
			t.Fatal("single observation lost")
		}
		assertGraphWidth(t, buf.String(), 40)
	}
	var buf bytes.Buffer
	if err := (Graph{Width: 24}).Print(&buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "No data") {
		t.Fatal("empty series fabricated observations")
	}
	points := make([]GraphPoint, 200)
	for i := range points {
		points[i] = GraphPoint{Time: start.Add(time.Duration(i) * time.Hour), Value: 0}
	}
	points[100].Value = 100
	buf.Reset()
	if err := (Graph{Width: 24, Points: points}).Print(&buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Split(buf.String(), "\n")[0], "●") {
		t.Fatal("dense series lost its peak")
	}
	assertGraphWidth(t, buf.String(), 24)
}

func TestGraphValidationAndWriterErrors(t *testing.T) {
	for _, graph := range []Graph{
		{Width: 23},
		{Width: 1001},
		{Width: -1},
		{Points: []GraphPoint{{Value: math.NaN()}}},
		{Points: []GraphPoint{{Value: math.Inf(1)}}},
		{Bars: true, Points: []GraphPoint{{Value: -1}}},
		{Points: []GraphPoint{{Value: 1}}},
	} {
		if err := graph.Print(&bytes.Buffer{}); err == nil {
			t.Fatalf("invalid graph accepted: %+v", graph)
		}
	}
	err := (Graph{}).Print(graphFailWriter{})
	if !errors.Is(err, errGraphWriter) {
		t.Fatalf("writer error lost: %v", err)
	}
}

func TestGraphAutomaticWidthAndPlainPipedOutput(t *testing.T) {
	t.Setenv("COLUMNS", "40")
	var buf bytes.Buffer
	if err := (Graph{Bars: true, Points: []GraphPoint{{Label: "Page", Value: 123}}}).Print(&buf); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "\x1b") {
		t.Fatal("piped graph contains colour escapes")
	}
	if runewidth.StringWidth(strings.TrimSuffix(buf.String(), "\n")) != 40 {
		t.Fatal("COLUMNS ignored")
	}
}

func assertGraphWidth(t *testing.T, text string, width int) {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		if runewidth.StringWidth(line) > width {
			t.Fatalf("line exceeds %d columns: %q", width, line)
		}
	}
}

var errGraphWriter = errors.New("writer failed")

type graphFailWriter struct{}

func (graphFailWriter) Write([]byte) (int, error) { return 0, errGraphWriter }
