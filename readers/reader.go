package readers

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strings"

	"github.com/dimchansky/utfbom"
)

type BasicAuthConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type HttpConfig struct {
	Insecure        *bool
	BasicAuthConfig *BasicAuthConfig
}

type LabelConfig struct {
	Col       int
	LabelName string
}

type CSVConfig struct {
	Name        string
	Url         url.URL
	TargetCol   int
	Labels      []LabelConfig
	Delimiter   string
	CommentChar string
	HttpConfig  *HttpConfig
}

type CSVReader interface {
	PrometheusTargets() ([]PrometheusTarget, error)
}

type PrometheusTarget struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels,omitempty"`
}

func stripComments(reader io.Reader, commentChar string) (io.Reader, error) {
	/*
		bodyBytes, _ := io.ReadAll(reader)
		trimmed := bytes.Trim(bodyBytes, "\xef\xbb\xbf")

		reader1 := strings.NewReader(string(trimmed))

	*/
	removeBomReader, enc := utfbom.Skip(reader)
	slog.Info("file encoding", "encoding", enc)
	r, w := io.Pipe()
	go func() {
		scanner := bufio.NewScanner(removeBomReader)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, commentChar) {
				_, err := fmt.Fprintln(w, line)
				if err != nil {
					slog.Error("scanner write", "error", err.Error())
				}
			}
		}
		_ = w.Close()
	}()
	return r, nil
}
