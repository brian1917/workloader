package utils

import (
	"encoding/csv"
	"fmt"
	"os"

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
			LogError(fmt.Sprintf("creating CSV - %s\n", err))
		}

		// Write CSV data
		writer := csv.NewWriter(outFile)
		writer.WriteAll(csvData)
		if err := writer.Error(); err != nil {
			LogError(fmt.Sprintf("writing CSV - %s\n", err))
		}
		// Log
		LogInfo(fmt.Sprintf("output file: %s", outFile.Name()), true)
	}
}
