package utils

import (
	"encoding/csv"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/viper"
)

// WriteOutput will write the CSV and/or stdout data based on the viper configuration
func WriteOutput(data [][]string, csvFileName string) {

	// Get the output format
	outFormat := viper.Get("output_format").(string)

	// Write CSV data if output format dictates it
	if outFormat == "csv" || outFormat == "both" {

		// Create CSV
		outFile, err := os.Create(csvFileName)
		if err != nil {
			Log(1, fmt.Sprintf("creating CSV - %s\n", err))
		}

		// Write CSV data
		writer := csv.NewWriter(outFile)
		writer.WriteAll(data)
		if err := writer.Error(); err != nil {
			Log(1, fmt.Sprintf("writing CSV - %s\n", err))
		}
		// Log
		fmt.Printf("Output file: %s\r\n\r\n", outFile.Name())
		Log(0, fmt.Sprintf("created %s", outFile.Name()))
	}

	// Write stdout if output format dictates it
	if outFormat == "stdout" || outFormat == "both" {
		table := tablewriter.NewWriter(os.Stdout)
		table.SetHeader(data[0])
		for i := 1; i <= len(data)-1; i++ {
			table.Append(data[i])
		}
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.Render()
	}
}
