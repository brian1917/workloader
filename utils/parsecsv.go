package utils

import (
	"bufio"
	"encoding/csv"
	"io"
	"os"
)

// ParseCSV parses a file and returns a slice of slice of strings
func ParseCSV(filename string) ([][]string, error) {

	// Open CSV File and create the reader
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(ClearBOM(bufio.NewReader(file)))

	// Create our slice to return
	var data [][]string

	// Iterate through CSV entries
	for {

		// Read the line
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		// Append
		data = append(data, line)
	}

	return data, nil
}
