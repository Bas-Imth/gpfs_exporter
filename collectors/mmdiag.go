// Copyright 2020 Trey Dockendorf
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collectors

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	configWaiterThreshold = kingpin.Flag("collector.mmdiag.waiter-threshold", "Threshold for collected waiters").Default("30").Int()
)

type DiagMetric struct {
	Waiters []DiagWaiter
}

type DiagWaiter struct {
	Seconds float64
	Thread  string
}

type MmdiagCollector struct {
	Waiter *prometheus.Desc
}

func init() {
	registerCollector("mmdiag", false, NewMmdiagCollector)
}

func NewMmdiagCollector() Collector {
	return &MmdiagCollector{
		Waiter: prometheus.NewDesc(prometheus.BuildFQName(namespace, "mmdiag", "waiter"),
			"GPFS max waiter in seconds", []string{"thread"}, nil),
	}
}

func (c *MmdiagCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.Waiter
}

func (c *MmdiagCollector) Collect(ch chan<- prometheus.Metric) {
	log.Debug("Collecting mmdiag metrics")
	err := c.collect(ch)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 1, "mmdiag")
	} else {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 0, "mmdiag")
	}
}

func (c *MmdiagCollector) collect(ch chan<- prometheus.Metric) error {
	collectTime := time.Now()
	out, err := mmdiag("--waiters")
	if err != nil {
		return err
	}
	var diagMetric DiagMetric
	err = parse_mmdiag_waiters(out, &diagMetric)
	if err != nil {
		return err
	}
	for _, waiter := range diagMetric.Waiters {
		ch <- prometheus.MustNewConstMetric(c.Waiter, prometheus.GaugeValue, waiter.Seconds, waiter.Thread)
	}
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "mmdiag")
	return nil
}

func mmdiag(arg string) (string, error) {
	cmd := execCommand("sudo", "/usr/lpp/mmfs/bin/mmdiag", arg, "-Y")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Error(err)
		return "", err
	}
	return out.String(), nil
}

func parse_mmdiag_waiters(out string, diagMetric *DiagMetric) error {
	lines := strings.Split(out, "\n")
	waitersPatter := regexp.MustCompile(`^Waiting ([0-9.]+) sec.*thread ([0-9]+)`)
	for _, l := range lines {
		match := waitersPatter.FindStringSubmatch(l)
		if len(match) != 3 {
			continue
		}
		secs, err := strconv.ParseFloat(match[1], 64)
		if err != nil {
			log.Errorf("Unable to convert %s to float64", match[1])
			continue
		}
		threshold := float64(*configWaiterThreshold)
		if secs >= threshold {
			waiter := DiagWaiter{Seconds: secs, Thread: match[2]}
			diagMetric.Waiters = append(diagMetric.Waiters, waiter)
		}
	}
	return nil
}