package echarts

import (
	"fmt"
	"strconv"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
)

func Bar(title string, XAxis interface{}, XName, YName string) *charts.Bar {
	bar := charts.NewBar()
	bar.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title: title,
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Name: XName,
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name:    YName,
			NameGap: 25,
			Show:    true,
		}),
		charts.WithGridOpts(opts.Grid{
			Top: "15%",
		}),
	)
	bar.SetSeriesOptions(
		charts.WithSeriesAnimation(true),
	)
	bar.SetXAxis(XAxis)
	return bar
}

func AddBarSeriesUint64(b *charts.Bar, name string, values []uint64) {
	data := make([]opts.BarData, 0)
	for _, v := range values {
		data = append(data, newBarData(v))
	}
	b.AddSeries(name, data, newBarOpts())
}

func AddBarSeriesFloat64(b *charts.Bar, name string, values []float64) {
	data := make([]opts.BarData, 0)
	for _, v := range values {
		v, _ = strconv.ParseFloat(fmt.Sprintf("%.2f", v), 64)
		data = append(data, newBarData(v))
	}
	b.AddSeries(name, data, newBarOpts())
}

func newBarData(v interface{}) opts.BarData {
	return opts.BarData{
		Value: v,
		Label: &opts.Label{
			Show:     true,
			Position: "top",
		},
	}
}

func newBarOpts() charts.SeriesOpts {
	return func(s *charts.SingleSeries) {
		s.Animation = true
	}
}
