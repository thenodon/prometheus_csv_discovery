package main

import (
	"encoding/json"
	"flag"
	"gopkg.in/yaml.v3"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"prometheus_csv_discovery/readers"
)

const (
	FILE  = "file"
	HTTP  = "http"
	HTTPS = "https"
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
	CSVSource   string        `yaml:"csv_source"`
	TargetCol   int           `yaml:"target_col"`
	Labels      []LabelConfig `yaml:"labels"`
	Delimiter   string        `yaml:"delimiter"`
	CommentChar string        `yaml:"comment_char"`
	HttpConfig  *HttpConfig   `yaml:"http_config,omitempty"`
}

var (
	reader readers.CSVReader
)

func handlePrometheusDiscovery(w http.ResponseWriter, _ *http.Request) {
	data, err := reader.PrometheusTargets()
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

func loadConfig(configPath string) (Config, error) {
	var config Config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return config, err
	}
	err = yaml.Unmarshal(data, &config)
	return config, err
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	configPath := flag.String("config", "config.yaml", "Path to the configuration file")
	flag.Parse()

	config, err := loadConfig(*configPath)
	if err != nil {
		slog.Error("read config file", slog.String("error", err.Error()))
		return
	}
	uri, err := url.Parse(config.CSVSource)
	if err != nil {
		slog.Error("not a valid url", slog.String("error", err.Error()))
		return
	}

	csvConfig := readers.CSVConfig{
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
		setupHttp(config, uri, csvConfig)
	} else {
		slog.Error("unsupported schema", slog.String("schema", uri.Scheme))
		return
	}

	http.HandleFunc("/prometheus/discovery", handlePrometheusDiscovery)
	addr := os.Getenv("SERVER_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	slog.Info("starting server", "addr", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		slog.Error("starting server failed", "error", err.Error())
	}
}

func setupFile(csvConfig readers.CSVConfig) {
	reader = readers.NewCSVFileReader(csvConfig)
}

func setupHttp(config Config, uri *url.URL, csvConfig readers.CSVConfig) {
	if config.HttpConfig != nil && uri.Scheme == HTTPS {
		defaultInsecure := true
		httpConfig := readers.HttpConfig{
			Insecure:        &defaultInsecure,
			BasicAuthConfig: nil,
		}
		csvConfig.HttpConfig = &httpConfig
		if config.HttpConfig.Insecure != nil {
			csvConfig.HttpConfig.Insecure = config.HttpConfig.Insecure
		}
	}

	if config.HttpConfig != nil && config.HttpConfig.BasicAuthConfig != nil {
		csvConfig.HttpConfig.BasicAuthConfig = &readers.BasicAuthConfig{
			Username: config.HttpConfig.BasicAuthConfig.Username,
			Password: config.HttpConfig.BasicAuthConfig.Password,
		}
	}
	reader = readers.NewCSVHttpReader(csvConfig)
}
