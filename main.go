package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/gukaifeng/rados-bench-plot/pkg/echarts"
	"github.com/gukaifeng/rados-bench-plot/pkg/util"
)

const (
	root       = "data"
	jsonSuffix = ".json"
	outputDir  = "output"
)

type BySize []string

func (a BySize) Len() int           { return len(a) }
func (a BySize) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a BySize) Less(i, j int) bool { return parseSize(a[i]) < parseSize(a[j]) }

func parseSize(s string) int {
	unit := s[len(s)-1]
	size, _ := strconv.Atoi(s[:len(s)-1])

	switch unicode.ToLower(rune(unit)) {
	case 'k':
		return size * 1024
	case 'm':
		return size * 1024 * 1024
	case 'g':
		return size * 1024 * 1024 * 1024
	default:
		return size
	}
}

type Metrics struct {
	IOPS    uint64
	Bw      float64
	Latency float64
}

type RADOSBenchData struct {
	// map[numjobs]map[blocksize]map[name]data
	data        map[string]map[string]map[string]Metrics
	bss         map[uint64]BySize
	items       map[uint64][]string
	IOPSData    map[string][]uint64
	BWData      map[string][]float64
	LatencyData map[string][]float64
}

type RADOSBenchOutputs map[string]*RADOSBenchOutput

type SecondData struct {
	Sec      string `json:"sec"`      // uint64
	CurOps   string `json:"cur_ops"`  // uint64
	Started  string `json:"started"`  // uint64
	Finished string `json:"finished"` // uint64
	AvgBw    string `json:"avg_bw"`   // float64
	CurBw    string `json:"cur_bw"`   // float64
	LastLat  string `json:"last_lat"` // float64
	AvgLat   string `json:"avg_lat"`  // float64
}

type RADOSBenchOutput struct {
	ConcurrentIOs   string       `json:"concurrent_ios"` // uint64
	ObjectSize      string       `json:"object_size"`    // uint64
	OpSize          string       `json:"op_size"`        // uint64
	SecondsToRun    string       `json:"seconds_to_run"` // uint64
	MaxObjects      string       `json:"max_objects"`    // uint64
	ObjectPrefix    string       `json:"object_prefix"`
	Datas           []SecondData `json:"datas"`
	TotalTimeRun    string       `json:"total_time_run"`    // float64
	TotalWritesMade string       `json:"total_writes_made"` // uint64
	WriteSize       string       `json:"write_size"`        // uint64
	Bandwidth       string       `json:"bandwidth"`         // float64
	StddevBandwidth string       `json:"stddev_bandwidth"`  // float64
	MaxBandwidth    string       `json:"max_bandwidth"`     // float64
	MinBandwidth    string       `json:"min_bandwidth"`     // float64
	AverageIOPS     string       `json:"average_iops"`      // uint64
	StddevIOPS      string       `json:"stddev_iops"`       // uint64
	MaxIOPS         string       `json:"max_iops"`          // uint64
	MinIOPS         string       `json:"min_iops"`          // uint64
	AverageLatency  string       `json:"average_latency"`   // float64
	StddevLatency   string       `json:"stddev_latency"`    // float64
	MaxLatency      string       `json:"max_latency"`       // float64
	MinLatency      string       `json:"min_latency"`       // float64
}

func (r RADOSBenchOutput) averageIOPS() uint64 {
	v, _ := strconv.ParseUint(r.AverageIOPS, 10, 64)
	return v
}

func (r RADOSBenchOutput) bandwidth() float64 {
	v, _ := strconv.ParseFloat(r.Bandwidth, 64)
	return v
}

func (r RADOSBenchOutput) averageLatency() float64 {
	v, _ := strconv.ParseFloat(r.AverageLatency, 64)
	return v
}

func (p RADOSBenchData) isSorted(numjobs uint64) bool {
	if p.bss[numjobs] != nil && len(p.bss[numjobs]) > 0 {
		return true
	}
	return false
}
func (p *RADOSBenchData) sort(numjobs uint64) {
	if p.isSorted(numjobs) {
		return
	}

	var (
		jobData map[string]map[string]Metrics
		ok      bool
	)
	if jobData, ok = p.data[strconv.FormatUint(numjobs, 10)]; !ok {
		return
	}

	itemSet := make(map[string]struct{})
	for bs, bsData := range jobData {
		p.bss[numjobs] = append(p.bss[numjobs], bs)
		for item := range bsData {
			if _, ok := itemSet[item]; !ok {
				p.items[numjobs] = append(p.items[numjobs], item)
				itemSet[item] = struct{}{}
			}
		}
	}

	sort.Sort(p.bss[numjobs])
	sort.Strings(p.items[numjobs])
}

func (p *RADOSBenchData) prepareData(numjobs uint64) {
	if p.IOPSData != nil || p.BWData != nil || p.LatencyData != nil {
		return
	}

	var (
		jobData map[string]map[string]Metrics
		ok      bool
	)
	if jobData, ok = p.data[strconv.FormatUint(numjobs, 10)]; !ok {
		return
	}

	p.sort(numjobs)

	p.IOPSData = make(map[string][]uint64)
	p.BWData = make(map[string][]float64)
	p.LatencyData = make(map[string][]float64)

	for _, bs := range p.bss[numjobs] {
		bsData := jobData[bs]
		for _, item := range p.items[numjobs] {
			matrics := bsData[item]
			p.IOPSData[item] = append(p.IOPSData[item], matrics.IOPS)
			p.BWData[item] = append(p.BWData[item], matrics.Bw)
			p.LatencyData[item] = append(p.LatencyData[item], matrics.Latency*1000.0)
		}
	}
}

func (p RADOSBenchData) plotIOPS(numjobs uint64) *charts.Bar {
	bar := echarts.Bar(fmt.Sprintf("RADOS Bench IOPS / Jobs = %d", numjobs), p.bss[numjobs], "Block Size", "IOPS")
	for _, item := range p.items[numjobs] {
		echarts.AddBarSeriesUint64(bar, item, p.IOPSData[item])
	}
	return bar
}

func (p RADOSBenchData) plotBW(numjobs uint64) *charts.Bar {
	bar := echarts.Bar(fmt.Sprintf("RADOS Bench BW / Jobs = %d", numjobs), p.bss[numjobs], "Block Size", "BW (MB/s)")
	for _, item := range p.items[numjobs] {
		echarts.AddBarSeriesFloat64(bar, item, p.BWData[item])
	}
	return bar
}

func (p RADOSBenchData) plotLatency(numjobs uint64) *charts.Bar {
	bar := echarts.Bar(fmt.Sprintf("RADOS Bench Latency / Jobs = %d", numjobs), p.bss[numjobs], "Block Size", "Latency (ms)")
	for _, item := range p.items[numjobs] {
		echarts.AddBarSeriesFloat64(bar, item, p.LatencyData[item])
	}
	return bar
}

func splitString(name string) (string, string, string) {
	parts := strings.Split(name, "_")
	if len(parts) < 3 {
		return "", "", ""
	}
	firstPart := strings.Join(parts[:len(parts)-2], "_")
	secondPart := parts[len(parts)-2]
	thirdPart := parts[len(parts)-1]
	return firstPart, secondPart, thirdPart
}

func detecteJsonFiles(root string) RADOSBenchOutputs {
	fmt.Println("detecting json file(s)...")
	rbs := make(RADOSBenchOutputs)
	dirs, err := os.ReadDir(root)
	util.AssertNoError(err)
	for i, d := range dirs {
		if d.IsDir() || !strings.HasSuffix(d.Name(), jsonSuffix) {
			continue
		}
		jsonPath := path.Join(root, d.Name())

		jsonContent := &RADOSBenchOutput{}
		c, err := os.ReadFile(jsonPath)
		util.AssertNoError(err)
		err = json.Unmarshal(c, jsonContent)
		util.AssertNoError(err)

		rbs[strings.TrimSuffix(d.Name(), jsonSuffix)] = jsonContent
		fmt.Printf("%5d json file(s) detected: %s\n", i+1, jsonPath)
	}
	return rbs
}

func parserJsonContent(rbs RADOSBenchOutputs) RADOSBenchData {
	plotData := RADOSBenchData{
		data:  make(map[string]map[string]map[string]Metrics),
		bss:   make(map[uint64]BySize),
		items: make(map[uint64][]string),
	}
	for name, content := range rbs {
		n, jobs, bs := splitString(name)
		m := Metrics{
			IOPS:    content.averageIOPS(),
			Bw:      content.bandwidth(),
			Latency: content.averageLatency(),
		}
		if _, ok := plotData.data[jobs]; !ok {
			plotData.data[jobs] = make(map[string]map[string]Metrics)
		}
		if _, ok := plotData.data[jobs][bs]; !ok {
			plotData.data[jobs][bs] = make(map[string]Metrics)
		}
		plotData.data[jobs][bs][n] = m
	}
	return plotData
}

func main() {
	rbs := detecteJsonFiles(root)
	plotData := parserJsonContent(rbs)

	fmt.Println("...")

	plotData.prepareData(1)
	iopsbar := plotData.plotIOPS(1)
	bwbar := plotData.plotBW(1)
	latencybar := plotData.plotLatency(1)

	page := components.NewPage()
	page.AddCharts(iopsbar)
	page.AddCharts(bwbar)
	page.AddCharts(latencybar)

	outputName := path.Join(outputDir, "rados-"+strconv.FormatInt(time.Now().Unix(), 10)+".html")
	output, err := os.OpenFile(outputName, os.O_RDWR|os.O_CREATE, 0666)
	util.AssertNoError(err)
	defer output.Close()

	page.Render(output)
	fmt.Printf("the results have been saved in %s\n", outputName)

	// http.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 	page.Render(w)
	// }))
	// log.Fatal(http.ListenAndServe(":9091", nil))
}
