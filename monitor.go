package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/klog"
)

var (
	addr   = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")
	period = flag.Int("period", 15, "The number of seconds to wait between a request cycle.")
)

var (
	requestDurations = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Name:       "external_monitor_durations_milliseconds",
			Help:       "External request monitor latency distributions.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
		[]string{"host"},
	)
	requestErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "external_monitor_error_count",
			Help: "External request monitor error count.",
		},
		[]string{"host"},
	)
	invalidCharacters = regexp.MustCompile(`[^A-Z_a-z]+`)
)

func init() {
	prometheus.MustRegister(requestDurations)
	prometheus.MustRegister(requestErrors)
	prometheus.MustRegister(prometheus.NewBuildInfoCollector())
}

func getMetricName(host string) string {
	return invalidCharacters.ReplaceAllString(host, "_")
}

func makeRequest(urlstring string) {
	urlstring = strings.Trim(urlstring, "\r ")
	if urlstring == "" {
		return
	}

	u, err := url.Parse(urlstring)
	if err != nil {
		panic("Bad url " + urlstring + err.Error())
	}

	metricName := getMetricName(u.Host)
	start := time.Now()
	resp, err := http.Get(urlstring)
	if err != nil {
		requestErrors.WithLabelValues(metricName).Inc()
		klog.Errorf("Error hitting %s (%s)\n", urlstring, metricName)
	}

	if resp != nil {
		defer resp.Body.Close()
	}

	elapsed := time.Since(start)
	elapsedMilliseconds := float64(elapsed) / float64(time.Millisecond)
	klog.Infof("Hit %s in %d ms\n", urlstring, int32(elapsedMilliseconds))
	requestDurations.WithLabelValues(metricName).Observe(elapsedMilliseconds)
}

func main() {
	flag.Parse()
	klog.InitFlags(nil)

	readText, err := ioutil.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("failed to read stdin: %s", err)
	}

	lines := strings.Split(string(readText), "\n")
	go func() {
		for {
			for _, urlstring := range lines {
				makeRequest(urlstring)
			}

			klog.Infof("Sleep for %d sec\n", *period)
			time.Sleep(time.Duration(*period) * time.Second)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*addr, nil))
}
