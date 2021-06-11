package main

import (
	"fmt"
	goruntime "runtime"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/guptarohit/asciigraph"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/node_exporter/collector"
	"github.com/testground/sdk-go/runtime"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

type measure struct {
	runenv       *runtime.RunEnv
	registry     *prometheus.Registry
	chStop       chan struct{}
	metrics      map[string][]float64
	lastRecv     float64
	lastTransmit float64
}

const (
	metricGoroutines    = "goroutines"
	metricHeapAllocMiBs = "heap-alloc-mibs"
	metricRecvBytes     = "receive-bytes"
	metricTransmitBytes = "transmit-bytes"
)

// startMeasure starts collecting number of goroutines, golang heap allocation and transmit/receive bytes every second, until stopAndPrint is called on the returned measure, at which point it sends all the recorded metrics as test result to InfluxDB, and prints them as line graphs for inspection.
func startMeasure(runenv *runtime.RunEnv) (*measure, error) {
	// have to do this because node_exporter requires it being called to properly initialize global variables.
	kingpin.Parse()
	logger := log.NewNopLogger()
	registry := prometheus.NewRegistry()
	collector.DisableDefaultCollectors()
	nodeCollector, err := collector.NewNodeCollector(logger)
	if err != nil {
		return nil, err
	}
	netCollector, err := collector.NewNetDevCollector(logger)
	if err != nil {
		return nil, err
	}
	nodeCollector.Collectors["net"] = netCollector
	registry.MustRegister(nodeCollector)
	p := &measure{runenv: runenv, registry: registry,
		chStop:  make(chan struct{}),
		metrics: make(map[string][]float64),
	}
	go func() {
		p.Collect()
	}()

	return p, nil
}

func (p *measure) Collect() {
	tk := time.NewTicker(time.Second)
	for {
		select {
		case <-tk.C:
			mf, err := p.registry.Gather()
			if err != nil {
				panic(err)
			}
			for _, m := range mf {
				switch *m.Name {
				case "node_network_receive_bytes_total":
					p.collectRecv(m.Metric)
				case "node_network_transmit_bytes_total":
					p.collectTransmit(m.Metric)
				}
			}
			p.collectGoroutines()
			p.collectMemStats()
		case <-p.chStop:
			return
		}
	}
}

func (p *measure) collectGoroutines() {
	p.metrics[metricGoroutines] = append(p.metrics[metricGoroutines], float64(goruntime.NumGoroutine()))
}

func (p *measure) collectMemStats() {
	var m goruntime.MemStats
	goruntime.ReadMemStats(&m)
	p.metrics[metricHeapAllocMiBs] = append(p.metrics[metricHeapAllocMiBs], float64(m.HeapAlloc)/1048576.0)
}

func (p *measure) collectRecv(metrics []*dto.Metric) {
	total := p.collectBytes(metrics)
	usage := total - p.lastRecv
	if p.lastRecv > 0 {
		p.metrics[metricRecvBytes] = append(p.metrics[metricRecvBytes], usage)
		p.runenv.D().Gauge(metricRecvBytes).Update(usage)
	}
	p.lastRecv = total
}

func (p *measure) collectTransmit(metrics []*dto.Metric) {
	total := p.collectBytes(metrics)
	usage := total - p.lastTransmit
	if p.lastTransmit > 0 {
		p.metrics[metricTransmitBytes] = append(p.metrics[metricTransmitBytes], usage)
		p.runenv.D().Gauge(metricTransmitBytes).Update(usage)
	}
	p.lastTransmit = total
}

func (p *measure) collectBytes(metrics []*dto.Metric) float64 {
	var total, exclude float64
	for _, m := range metrics {
		total += *m.Counter.Value
		for _, label := range m.Label {
			if *label.Name == "device" && *label.Value == "lo0" {
				exclude += *m.Counter.Value
			}
		}
	}
	return total - exclude
}

func (p *measure) stopAndPrint() {
	close(p.chStop)
	output := fmt.Sprintf("Test params: %v", p.runenv.TestInstanceParams)
	for _, name := range []string{
		metricGoroutines,
		metricHeapAllocMiBs,
		metricRecvBytes,
		metricTransmitBytes,
	} {
		if len(p.metrics[name]) == 0 {
			p.runenv.RecordMessage("WARNING: No metrics for %s!", name)
			continue
		}
		output += "\n"
		output += asciigraph.Plot(
			p.metrics[name],
			asciigraph.Caption(name),
			asciigraph.Width(100),
			asciigraph.Height(10),
			asciigraph.Offset(10))
	}
	p.runenv.RecordMessage(output)
}
