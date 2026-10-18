package output

import (
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mattn/go-runewidth"
	"golang.org/x/term"
)

// GraphPoint is an observation or a ranked value. Time is required for line charts.
type GraphPoint struct {
	Label string
	Value float64
	Time  time.Time
}

// Graph renders a time series or horizontal ranking without changing the data.
type Graph struct {
	Title  string
	Points []GraphPoint
	Bars   bool
	// Width is measured in terminal columns; zero detects the terminal size.
	Width int
}

// Print writes a labelled graph. Empty datasets produce a message, not fake zeros.
func (g Graph) Print(writer io.Writer) error {
	width := g.Width
	if width == 0 {
		if file, ok := writer.(*os.File); ok {
			width, _, _ = term.GetSize(int(file.Fd()))
		}
		if width <= 0 {
			width, _ = strconv.Atoi(os.Getenv("COLUMNS"))
		}
		if width <= 0 {
			width = 80
		}
	}
	if width < 24 {
		return fmt.Errorf("graphs need at least 24 columns; use --graph-width 80 or --output table")
	}
	if width > 1000 {
		return fmt.Errorf("--graph-width must not exceed 1000 columns")
	}
	colour := false
	if file, ok := writer.(*os.File); ok && os.Getenv("NO_COLOR") == "" {
		colour = term.IsTerminal(int(file.Fd()))
	}
	paint := func(text string) string {
		if colour {
			return "\x1b[36m" + text + "\x1b[0m"
		}
		return text
	}
	var result strings.Builder
	if g.Title != "" {
		fmt.Fprintln(&result, runewidth.Truncate(graphLabel(g.Title), width, "…"))
	}
	for _, point := range g.Points {
		if math.IsNaN(point.Value) || math.IsInf(point.Value, 0) {
			return fmt.Errorf("graph values must be finite numbers")
		}
		if g.Bars && point.Value < 0 {
			return fmt.Errorf("bar graphs require non-negative values")
		}
		if !g.Bars && point.Time.IsZero() {
			return fmt.Errorf("line graphs require observation dates")
		}
	}
	if len(g.Points) == 0 {
		fmt.Fprintln(&result, "No data for this period.")
	} else if g.Bars {
		g.barGraph(&result, width, paint)
	} else {
		if err := g.lineGraph(&result, width, paint); err != nil {
			return err
		}
	}
	_, err := io.WriteString(writer, result.String())
	return err
}

func (g Graph) barGraph(result *strings.Builder, width int, paint func(string) string) {
	labelWidth, valueWidth, maxValue := 0, 0, 0.0
	for _, point := range g.Points {
		labelWidth = max(labelWidth, runewidth.StringWidth(graphLabel(point.Label)))
		valueWidth = max(valueWidth, len(graphNumber(point.Value)))
		maxValue = max(maxValue, point.Value)
	}
	labelWidth = max(1, min(labelWidth, width/3))
	barWidth := width - labelWidth - valueWidth - 4
	for _, point := range g.Points {
		label := runewidth.Truncate(graphLabel(point.Label), labelWidth, "…")
		size := 0
		if maxValue > 0 && point.Value > 0 {
			size = max(1, int(math.Round(point.Value/maxValue*float64(barWidth))))
		}
		fmt.Fprintf(result, "%s%s  %s%s  %*s\n", label, strings.Repeat(" ", labelWidth-runewidth.StringWidth(label)), paint(strings.Repeat("█", size)), strings.Repeat(" ", barWidth-size), valueWidth, graphNumber(point.Value))
	}
}

func (g Graph) lineGraph(result *strings.Builder, width int, paint func(string) string) error {
	points := append([]GraphPoint(nil), g.Points...)
	sort.SliceStable(points, func(i, j int) bool { return points[i].Time.Before(points[j].Time) })
	low, high := points[0].Value, points[0].Value
	for _, point := range points {
		low = min(low, point.Value)
		high = max(high, point.Value)
	}
	if low == high {
		padding := max(1, math.Abs(low)*0.05)
		low -= padding
		high += padding
		if points[0].Value >= 0 {
			low = max(0, low)
		}
	}
	if math.IsInf(high-low, 0) || math.IsInf(high, 0) || math.IsInf(low, 0) {
		return fmt.Errorf("value range is too large to plot")
	}
	const height = 8
	ticks := make([]string, height)
	labelWidth := 0
	for y := range height {
		ticks[y] = graphNumber(high - float64(y)*(high-low)/float64(height-1))
		labelWidth = max(labelWidth, len(ticks[y]))
	}
	// Abbreviations must not hide a small change between large counts.
	duplicateTicks := false
	for y := 1; y < height; y++ {
		duplicateTicks = duplicateTicks || ticks[y] == ticks[y-1]
	}
	if duplicateTicks {
		labelWidth = 0
		for y := range height {
			ticks[y] = strconv.FormatFloat(high-float64(y)*(high-low)/float64(height-1), 'f', 1, 64)
			labelWidth = max(labelWidth, len(ticks[y]))
		}
	}
	plotWidth := width - labelWidth - 2
	if plotWidth < 10 {
		return fmt.Errorf("graph labels need more columns; increase --graph-width")
	}
	grid := make([][]rune, height)
	for y := range height {
		grid[y] = []rune(strings.Repeat(" ", plotWidth))
	}
	first, last := points[0].Time, points[len(points)-1].Time
	span := last.Sub(first).Seconds()
	previousX, previousY := 0, 0
	positions := make([][2]int, 0, len(points))
	for i, point := range points {
		x := plotWidth / 2
		if span > 0 {
			x = int(math.Round(point.Time.Sub(first).Seconds() / span * float64(plotWidth-1)))
		}
		y := int(math.Round((high - point.Value) / (high - low) * float64(height-1)))
		x = max(0, min(plotWidth-1, x))
		y = max(0, min(height-1, y))
		if i > 0 {
			graphLine(grid, previousX, previousY, x, y)
		}
		positions = append(positions, [2]int{x, y})
		previousX, previousY = x, y
	}
	// Draw observations last so an interpolated segment cannot erase a point.
	for _, position := range positions {
		grid[position[1]][position[0]] = '●'
	}
	for y, row := range grid {
		fmt.Fprintf(result, "%*s ┤%s\n", labelWidth, ticks[y], paint(string(row)))
	}
	prefix := strings.Repeat(" ", labelWidth+2)
	fmt.Fprintf(result, "%s└%s\n", strings.Repeat(" ", labelWidth+1), strings.Repeat("─", plotWidth))
	start, end := first.Format("2006-01-02"), last.Format("2006-01-02")
	switch {
	case first.Equal(last):
		fmt.Fprintln(result, prefix+start)
	case plotWidth >= len(start)+len(end)+1:
		fmt.Fprintln(result, prefix+start+strings.Repeat(" ", plotWidth-len(start)-len(end))+end)
	default:
		fmt.Fprintln(result, prefix+start)
		fmt.Fprintln(result, prefix+strings.Repeat(" ", plotWidth-len(end))+end)
	}
	// Multiple observations in a terminal column retain their range through the
	// vertical segments above. Missing dates are never replaced with zero.
	noun := "observations"
	if len(points) == 1 {
		noun = "observation"
	}
	fmt.Fprintf(result, "%d %s\n", len(points), noun)
	fmt.Fprintln(result, "Gaps aren't zero-filled.")
	return nil
}

// Bresenham's integer line preserves peaks even when samples share a column.
func graphLine(grid [][]rune, x0, y0, x1, y1 int) {
	dx, dy := int(math.Abs(float64(x1-x0))), -int(math.Abs(float64(y1-y0)))
	sx, sy := -1, -1
	if x0 < x1 {
		sx = 1
	}
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		glyph := '─'
		if dx == 0 {
			glyph = '│'
		} else if dy != 0 {
			glyph = '╱'
			if sy > 0 {
				glyph = '╲'
			}
		}
		grid[y0][x0] = glyph
		if x0 == x1 && y0 == y1 {
			return
		}
		twice := 2 * err
		if twice >= dy {
			err += dy
			x0 += sx
		}
		if twice <= dx {
			err += dx
			y0 += sy
		}
	}
}

func graphNumber(value float64) string {
	if math.Abs(value) >= 1e15 || (value != 0 && math.Abs(value) < 0.1) {
		return strconv.FormatFloat(value, 'g', 3, 64)
	}
	for _, unit := range []struct {
		size  float64
		label string
	}{{1e12, "T"}, {1e9, "B"}, {1e6, "M"}, {1e3, "K"}} {
		if math.Abs(value) >= unit.size {
			return strings.TrimSuffix(strconv.FormatFloat(value/unit.size, 'f', 1, 64), ".0") + unit.label
		}
	}
	return strings.TrimSuffix(strconv.FormatFloat(value, 'f', 1, 64), ".0")
}

func graphLabel(value string) string {
	return strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)), " ")
}
