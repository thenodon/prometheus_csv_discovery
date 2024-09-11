package readers

import (
	"encoding/csv"
	"github.com/fsnotify/fsnotify"
	"log/slog"
	"os"
	"sync"
)

func NewCSVFileReader(config CSVConfig) *CSVFileReader {

	reader := CSVFileReader{CSVConfig: config}
	go reader.watchFile()
	return &reader
}

type CSVFileReader struct {
	CSVConfig CSVConfig
	mu        sync.RWMutex
	targets   []PrometheusTarget
}

func (c *CSVFileReader) watchFile() {

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("create watcher", "error", err.Error())
		return
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					slog.Info("modified file", "event", event.Name)
					err = c.reRead()
					if err != nil {
						slog.Error("reRead", "error", err.Error())
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}

				slog.Error("select watcher", "error", err.Error())
			}
		}
	}()
	filePath := c.CSVConfig.Url.Path
	err = watcher.Add(filePath)
	if err != nil {
		slog.Error("add watcher file", "error", err.Error())
		return
	}
	<-done
}

func (c *CSVFileReader) Read() ([][]string, error) {
	filePath := c.CSVConfig.Url.Path
	file, err := os.Open(filePath)
	if err != nil {
		slog.Error("open file", "file", filePath, "error", err.Error())
		return nil, err
	}
	defer file.Close()

	strippedReader, err := stripComments(file, c.CSVConfig.CommentChar)
	if err != nil {
		return nil, err
	}
	/*
		bodyBytes, err := io.ReadAll(strippedReader)
		bodyString := string(bodyBytes)
		fmt.Println(bodyString)
	*/
	reader := csv.NewReader(strippedReader)
	reader.Comma = rune(c.CSVConfig.Delimiter[0])
	return reader.ReadAll()
}

func (c *CSVFileReader) PrometheusTargets() ([]PrometheusTarget, error) {
	if c.targets == nil {
		err := c.reRead()
		if err != nil {
			slog.Error("reRead", "error", err.Error())
		}

	}
	return c.targets, nil
}

func (c *CSVFileReader) reRead() error {
	csvData, err := c.Read()
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
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

	return nil

}
