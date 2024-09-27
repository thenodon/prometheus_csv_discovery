package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"prometheus_csv_discovery/readers"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/ksuid"

	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v3"
)

const (
	FILE  = "file"
	HTTP  = "http"
	HTTPS = "https"

	MetricsPrefix     = "csv_discovery_"
	LogFieldRequestID = "requestid"
	LogFieldExecTime  = "exec_time"
)

type BasicAuthConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type HttpConfig struct {
	Insecure        *bool            `yaml:"insecure,omitempty"`
	BasicAuthConfig *BasicAuthConfig `yaml:"basic_auth,omitempty"`
}

type LabelConfig struct {
	Col       int    `yaml:"col"`
	LabelName string `yaml:"label_name"`
}

type Config struct {
	Name        string        `yaml:"name"`
	CSVSource   string        `yaml:"csv_source"`
	TargetCol   int           `yaml:"target_col"`
	Labels      []LabelConfig `yaml:"labels"`
	Delimiter   string        `yaml:"delimiter"`
	CommentChar string        `yaml:"comment_char"`
	HttpConfig  *HttpConfig   `yaml:"http_config,omitempty"`
}

type DiscoveryTargets struct {
	Configs []Config `yaml:"discovery_targets"`
}

var (
	allReaders map[string]readers.CSVReader
	version    = "undefined"
)

func handlePrometheusDiscovery(w http.ResponseWriter, r *http.Request) {
	queryParam := r.URL.Query().Get("discover")
	discovery, exists := allReaders[queryParam]
	if !exists {
		http.Error(w, "No such discovery", http.StatusNotFound)
		return
	}
	data, err := discovery.PrometheusTargets()
	if err != nil {
		http.Error(w, "Failed to get Prometheus targets", http.StatusInternalServerError)
		return
	}
	output, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "Failed to marshal JSON", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(output)
}

func loadConfig(configPath string) (DiscoveryTargets, error) {
	var config DiscoveryTargets
	data, err := os.ReadFile(configPath)
	if err != nil {
		return config, err
	}
	err = yaml.Unmarshal(data, &config)
	return config, err
}

func main() {

	versionFlag := flag.Bool("v", false, "Show version")
	configPath := flag.String("config", "config.yaml", "Path to the configuration file")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("prometheus-csv-discovery, version %s\n", version)
		os.Exit(0)
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

	fullConfiguration, err := loadConfig(*configPath)
	if err != nil {
		slog.Error("read config file", slog.String("error", err.Error()))
		return
	}
	allReaders = make(map[string]readers.CSVReader)
	// Setup readers
	for _, config := range fullConfiguration.Configs {
		uri, err := url.Parse(config.CSVSource)
		if err != nil {
			slog.Error("not a valid url", slog.String("error", err.Error()))
			return
		}

		csvConfig := readers.CSVConfig{
			Name:        config.Name,
			Url:         *uri,
			TargetCol:   config.TargetCol,
			Labels:      make([]readers.LabelConfig, len(config.Labels)),
			Delimiter:   config.Delimiter,
			CommentChar: config.CommentChar,
		}

		for i, label := range config.Labels {
			csvConfig.Labels[i] = readers.LabelConfig{
				Col:       label.Col,
				LabelName: label.LabelName,
			}
		}

		if uri.Scheme == FILE {
			setupFile(csvConfig)
		} else if uri.Scheme == HTTP || uri.Scheme == HTTPS {
			defaultInsecure := true
			httpConfig := readers.HttpConfig{
				Insecure:        &defaultInsecure,
				BasicAuthConfig: nil,
			}
			csvConfig.HttpConfig = &httpConfig
			if config.HttpConfig != nil {
				if config.HttpConfig.Insecure != nil {
					csvConfig.HttpConfig.Insecure = config.HttpConfig.Insecure
				}

				if config.HttpConfig != nil && config.HttpConfig.BasicAuthConfig != nil {
					csvConfig.HttpConfig.BasicAuthConfig = &readers.BasicAuthConfig{
						Username: config.HttpConfig.BasicAuthConfig.Username,
						Password: config.HttpConfig.BasicAuthConfig.Password,
					}
				}
			}
			setupHttp(csvConfig)
		} else {
			slog.Error("unsupported schema", slog.String("schema", uri.Scheme))
			return
		}
	}

	responseTime := promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: MetricsPrefix + "request_duration_seconds",
		Help: "Histogram of the time (in seconds) each request took to complete.",
		//Buckets:                     []float64{0.050, 0.100, 0.200, 0.500, 0.800, 1.00, 2.000, 3.000},
		NativeHistogramBucketFactor:     1.1,
		NativeHistogramMaxBucketNumber:  160,
		NativeHistogramMinResetDuration: time.Hour,
	},
		[]string{"url", "status"},
	)

	http.Handle("/prometheus-sd-targets", logCall(promMonitor(http.HandlerFunc(handlePrometheusDiscovery), responseTime, "/prometheus/discovery")))
	//http.HandleFunc("/prometheus/discovery", handlePrometheusDiscovery)
	http.Handle("/metrics", promhttp.HandlerFor(
		prometheus.DefaultGatherer,
		promhttp.HandlerOpts{
			// Opt into OpenMetrics to support exemplars.
			EnableOpenMetrics: true,
		},
	))

	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = ":9911"
	}
	slog.Info("starting server", "addr", addr, "version", version)
	if err := http.ListenAndServe(addr, nil); err != nil {
		slog.Error("starting server failed", "error", err.Error())
	}
}

func setupFile(csvConfig readers.CSVConfig) {
	allReaders[csvConfig.Name] = readers.NewCSVFileReader(csvConfig)
}

func setupHttp(csvConfig readers.CSVConfig) {
	allReaders[csvConfig.Name] = readers.NewCSVHttpReader(csvConfig)
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	length     int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func (lrw *loggingResponseWriter) Write(b []byte) (int, error) {
	if lrw.statusCode == 0 {
		lrw.statusCode = http.StatusOK
	}
	n, err := lrw.ResponseWriter.Write(b)
	lrw.length += n
	return n, err
}

func nextRequestID() ksuid.KSUID {
	return ksuid.New()
}

func logCall(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		start := time.Now()

		lrw := loggingResponseWriter{ResponseWriter: w}
		requestId := nextRequestID()

		ctx := context.WithValue(r.Context(), LogFieldRequestID, requestId)
		next.ServeHTTP(&lrw, r.WithContext(ctx)) // call original

		w.Header().Set("Content-Length", strconv.Itoa(lrw.length))
		slog.Info("api call",
			"method", r.Method,
			"uri", r.RequestURI,
			"status", lrw.statusCode,
			"length", lrw.length,
			LogFieldRequestID, ctx.Value(LogFieldRequestID),
			LogFieldExecTime, time.Since(start).Microseconds())
	})
}

func promMonitor(next http.Handler, ops *prometheus.HistogramVec, endpoint string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		start := time.Now()
		lrw := loggingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(&lrw, r) // call original
		response := time.Since(start).Seconds()
		ops.With(prometheus.Labels{"url": endpoint, "status": strconv.Itoa(lrw.statusCode)}).Observe(response)
	})
}
