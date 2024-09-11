package readers

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/csv"
	"net/http"
)

func NewCSVHttpReader(config CSVConfig) *CSVHttpReader {

	reader := CSVHttpReader{CSVConfig: config}
	reader.insecure = true
	reader.basicAuthConfig = nil
	return &reader
}

type CSVHttpReader struct {
	CSVConfig       CSVConfig
	targets         []PrometheusTarget
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

	strippedReader, err := stripComments(resp.Body, c.CSVConfig.CommentChar)
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(strippedReader)
	reader.Comma = rune(c.CSVConfig.Delimiter[0])
	r, err := reader.ReadAll()
	return r, err
}

func (c *CSVHttpReader) PrometheusTargets() ([]PrometheusTarget, error) {
	csvData, err := c.Read()
	if err != nil {
		return nil, err
	}

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
		target := PrometheusTarget{
			Targets: []string{row[c.CSVConfig.TargetCol]},
			Labels:  labels,
		}
		c.targets = append(c.targets, target)
	}
	return c.targets, nil
}
