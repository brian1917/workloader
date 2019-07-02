package mode

import (
	"bufio"
	"encoding/csv"
	"io"
	"log"
	"os"
)

type target struct {
	workloadHref string
	targetMode   string
}

func parseCsv(filename string) []target {
	var targets []target

	// Open CSV File
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("ERROR - opening CSV - %s", err)
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
			log.Fatalf("ERROR - reading CSV file - %s", err)
		}

		// Skipe the header row
		if i == 1 {
			continue
		}

		// Append to our array
		if line[1] != "idle" && line[1] != "build" && line[1] != "test" && line[1] != "enforced" {
			log.Fatalf("ERROR - invalid mode on line %d", i)
		}
		targets = append(targets, target{workloadHref: line[0], targetMode: line[1]})
	}

	return targets
}
