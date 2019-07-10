package mode

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"

	"github.com/brian1917/workloader/utils"
)

type target struct {
	workloadHref string
	targetMode   string
}

func parseCsv(filename string) []target {
	// Adjust the columns so first column is really 0
	hrefCol--
	desiredStateCol--

	var targets []target

	// Open CSV File
	file, err := os.Open(filename)
	if err != nil {
		utils.Logger.Fatalf("[ERROR] - opening CSV - %s", err)
	}
	defer file.Close()

	reader := csv.NewReader(bufio.NewReader(file))

	// Start the counters
	i := 0

	// Iterate through CSV entries
	for {

		// Increment the counter
		i++

		// Read the line
		line, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			fmt.Println("Error - see workloader.log file")
			utils.Logger.Fatalf("[ERROR] - reading CSV file - %s", err)
		}

		// Skipe the header row
		if i == 1 {
			continue
		}

		// Append to our array
		if line[desiredStateCol] != "idle" && line[desiredStateCol] != "build" && line[desiredStateCol] != "test" && line[desiredStateCol] != "enforced" {
			fmt.Println("Error - see workloader.log file")
			utils.Logger.Fatalf("[ERROR] - invalid mode on line %d - %s not acceptable", i, line[desiredStateCol])
		}
		targets = append(targets, target{workloadHref: line[hrefCol], targetMode: line[desiredStateCol]})
	}

	return targets
}
