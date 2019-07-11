package mode

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/brian1917/workloader/utils"
)

type target struct {
	workloadHref string
	targetMode   string
}

func parseCsv(filename string) []target {
	// Adjust the columns so first column is  0
	hrefCol--
	desiredStateCol--

	// Create our targets slice to hold results
	var targets []target

	// Open CSV File and create the reader
	file, err := os.Open(filename)
	if err != nil {
		utils.Log(1, fmt.Sprintf("opening CSV - %s", err))
	}
	defer file.Close()
	reader := csv.NewReader(bufio.NewReader(file))

	// Start the counter
	i := 0

	// Iterate through CSV entries
	for {

		// Read the line
		line, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			utils.Log(1, fmt.Sprintf("reading CSV file - %s", err))
		}

		// Increment the counter
		i++

		// Skipe the header row
		if i == 1 {
			continue
		}

		// Check to make sure we have a valid build state and then append to targets slice
		m := strings.ToLower(line[desiredStateCol])
		if m != "idle" && m != "build" && m != "test" && m != "enforced" {
			utils.Log(1, fmt.Sprintf("invalid mode on line %d - %s not acceptable", i, line[desiredStateCol]))
		}
		targets = append(targets, target{workloadHref: line[hrefCol], targetMode: m})
	}

	return targets
}
