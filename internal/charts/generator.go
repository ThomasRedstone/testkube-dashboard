package charts

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/testkube/dashboard/internal/database"
)

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

func (g *Generator) PassRateChart(data []database.DataPoint) string {
	line := charts.NewLine()
	line.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{Title: "Pass Rate Trend"}),
		charts.WithTooltipOpts(opts.Tooltip{Show: opts.Bool(true)}),
		charts.WithLegendOpts(opts.Legend{Show: opts.Bool(false)}),
		charts.WithInitializationOpts(opts.Initialization{
			Height: "200px", // Reduced height
			Width: "100%",   // Responsive width
		}),
	)

	xAxis := make([]string, len(data))
	yAxis := make([]opts.LineData, len(data))

	for i, dp := range data {
		xAxis[i] = dp.Date.Format("Jan 02")
		yAxis[i] = opts.LineData{Value: dp.PassRate}
	}

	line.SetXAxis(xAxis).
		AddSeries("Pass Rate %", yAxis).
		SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{Smooth: opts.Bool(true)}))

	return g.renderToString(line)
}

func (g *Generator) DurationChart(data []database.DataPoint) string {
	bar := charts.NewBar()
	bar.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{Title: "Test Duration Trend"}),
		charts.WithInitializationOpts(opts.Initialization{
			Height: "200px", // Reduced height
			Width: "100%",   // Responsive width
		}),
	)

	xAxis := make([]string, len(data))
	avgData := make([]opts.BarData, len(data))
	p95Data := make([]opts.BarData, len(data))

	for i, dp := range data {
		xAxis[i] = dp.Date.Format("Jan 02")
		avgData[i] = opts.BarData{Value: dp.AvgDuration}
		p95Data[i] = opts.BarData{Value: dp.P95Duration}
	}

	bar.SetXAxis(xAxis).
		AddSeries("Average", avgData).
		AddSeries("P95", p95Data)

	return g.renderToString(bar)
}

func (g *Generator) Sparkline(values []float64) string {
	if len(values) == 0 {
		return ""
	}
	width := 100
	height := 30

	min, max := values[0], values[0]
	for _, v := range values {
		if v < min { min = v }
		if v > max { max = v }
	}

	if min == max {
		max = min + 1
	}

	points := make([]string, len(values))
	for i, v := range values {
		x := float64(i) * float64(width) / float64(len(values)-1)
		y := float64(height) - ((v - min) / (max - min) * float64(height))
		points[i] = fmt.Sprintf("%.1f,%.1f", x, y)
	}

	polyline := strings.Join(points, " ")

	return fmt.Sprintf(`
		<svg width="%d" height="%d" class="sparkline">
			<polyline points="%s"
					  fill="none"
					  stroke="currentColor"
					  stroke-width="2"/>
		</svg>
	`, width, height, polyline)
}

// Interface for anything that can render itself to an io.Writer
type Renderer interface {
	Render(w io.Writer) error
}

func (g *Generator) renderToString(c Renderer) string {
	var buf bytes.Buffer
	c.Render(&buf)
	return buf.String()
}
