package utils

import (
	"encoding/csv"
	"fmt"
	"os"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/viper"
)

// WriteOutput will write the CSV and/or stdout data based on the viper configuration
func WriteOutput(csvData, stdOutData [][]string, csvFileName string) {

	// Get the output format
	outFormat := viper.Get("output_format").(string)

	// Write stdout if output format dictates it
	if outFormat == "stdout" || outFormat == "both" {
		if len(stdOutData) < viper.Get("max_entries_for_stdout").(int) {
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader(stdOutData[0])
			for i := 1; i <= len(stdOutData)-1; i++ {
				table.Append(stdOutData[i])
			}
			table.SetAlignment(tablewriter.ALIGN_LEFT)
			table.SetRowLine(true)
			table.Render()
		}
	}

	// Write CSV data if output format dictates it
	if outFormat == "csv" || outFormat == "both" {

		// Create CSV
		outFile, err := os.Create(csvFileName)
		if err != nil {
			LogError(fmt.Sprintf("creating csv - %s\n", err))
		}

		// Write CSV data
		writer := csv.NewWriter(outFile)
		writer.WriteAll(csvData)
		if err := writer.Error(); err != nil {
			LogError(fmt.Sprintf("writing csv - %s\n", err))
		}
		// Log
		LogInfo(fmt.Sprintf("output file: %s", outFile.Name()), true)
	}
}

// WriteLineOutput will write the CSV one line at a time
func WriteLineOutput(csvLine []string, csvFileName string) {

	var outFile *os.File

	// Create CSV if it doesn't exist
	if _, err := os.Stat(csvFileName); err != nil {
		outFile, err = os.Create(csvFileName)
		if err != nil {
			LogError(fmt.Sprintf("creating csv - %s\n", err))
		}
		LogInfo(fmt.Sprintf("output file started: %s", outFile.Name()), true)

	} else {
		outFile, err = os.OpenFile(csvFileName, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			LogError(fmt.Sprintf("opening csv - %s\n", err))
		}

	}
	defer outFile.Close()

	// Write CSV data
	writer := csv.NewWriter(outFile)
	defer writer.Flush()

	if err := writer.Write(csvLine); err != nil {
		LogError(fmt.Sprintf("error writing csv line - %s", err))
	}
}

func FileName(suffix string) string {
	if suffix != "" {
		return fmt.Sprintf("workloader-%s-%s-%s.csv", os.Args[1], suffix, time.Now().Format("20060102_150405"))
	}
	return fmt.Sprintf("workloader-%s-%s.csv", os.Args[1], time.Now().Format("20060102_150405"))
}
