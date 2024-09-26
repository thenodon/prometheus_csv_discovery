package readers

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/csv"
	"io"
	"net/http"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

func NewCSVHttpReader(config CSVConfig) *CSVHttpReader {

	reader := CSVHttpReader{CSVConfig: config}
	reader.insecure = true
	reader.basicAuthConfig = nil
	return &reader
}

type CSVHttpReader struct {
	CSVConfig CSVConfig
	//targets         []PrometheusTarget
	insecure        bool
	basicAuthConfig *BasicAuthConfig
}

func (c *CSVHttpReader) Read() ([][]string, error) {
	client := &http.Client{}
	if c.insecure {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		client.Transport = tr
	}

	req, err := http.NewRequest("GET", c.CSVConfig.Url.String(), nil)
	if err != nil {
		return nil, err
	}

	if c.basicAuthConfig != nil {
		auth := c.basicAuthConfig.Username + ":" + c.basicAuthConfig.Password
		encodedAuth := base64.StdEncoding.EncodeToString([]byte(auth))
		req.Header.Add("Authorization", "Basic "+encodedAuth)
	}

	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read a small portion of the file to detect encoding
	buf := make([]byte, 1024)
	n, err := resp.Body.Read(buf)
	if err != nil && err != io.EOF {
		return nil, err
	}

	// Reset the response body reader
	resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(buf[:n]), resp.Body))

	// Check for UTF-16 BOM
	if n >= 2 && (buf[0] == 0xFF && buf[1] == 0xFE) {
		// UTF-16 Little Endian
		resp.Body = io.NopCloser(transform.NewReader(resp.Body, unicode.UTF16(unicode.LittleEndian, unicode.ExpectBOM).NewDecoder()))
	} else if n >= 2 && (buf[0] == 0xFE && buf[1] == 0xFF) {
		// UTF-16 Big Endian
		resp.Body = io.NopCloser(transform.NewReader(resp.Body, unicode.UTF16(unicode.BigEndian, unicode.ExpectBOM).NewDecoder()))
	}

	strippedReader, err := stripComments(resp.Body, c.CSVConfig.CommentChar)
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(strippedReader)
	if c.CSVConfig.Delimiter == "" {
		reader.Comma = ','
	} else {
		reader.Comma = rune(c.CSVConfig.Delimiter[0])
	}
	r, err := reader.ReadAll()
	return r, err
}

func (c *CSVHttpReader) PrometheusTargets() ([]PrometheusTarget, error) {
	csvData, err := c.Read()
	if err != nil {
		return nil, err
	}

	targets := make([]PrometheusTarget, 0)
	for _, row := range csvData {
		if len(row) <= c.CSVConfig.TargetCol {
			continue
		}
		labels := make(map[string]string)
		for _, labelConfig := range c.CSVConfig.Labels {
			if len(row) > labelConfig.Col {
				labels[labelConfig.LabelName] = row[labelConfig.Col]
			}
		}
		target := PrometheusTarget{}
		if len(labels) == 0 {
			target.Targets = []string{row[c.CSVConfig.TargetCol]}

		} else {
			target.Targets = []string{row[c.CSVConfig.TargetCol]}
			target.Labels = labels
		}
		targets = append(targets, target)
	}
	return targets, nil
}
