package readers

import (
	"encoding/csv"
	"log/slog"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
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

	reader := csv.NewReader(strippedReader)
	if c.CSVConfig.Delimiter == "" {
		reader.Comma = ','
	} else {
		reader.Comma = rune(c.CSVConfig.Delimiter[0])
	}

	// if header is true, read the first line and ignore it
	if c.CSVConfig.Header {
		_, err = reader.Read()
		if err != nil {
			slog.Error("read header", "error", err.Error())
		}
	}

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

	c.targets = parseTargets(csvData, &c.CSVConfig)

	return nil

}
