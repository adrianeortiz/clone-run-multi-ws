package mapping

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/adrianeortiz/clone-run-multi-ws/qase"
)

// Mode represents the mapping mode
type Mode string

const (
	ModeCSV = "csv"
	ModeCF  = "custom_field"
)

// Build creates a mapping from source case ID to target case ID
func Build(mode Mode, srcCases map[int]qase.Case, tgtCases map[int]qase.Case, cfID int, csvPath string) (map[int]int, error) {
	switch mode {
	case ModeCSV:
		return buildCSVMapping(csvPath)
	case ModeCF:
		return buildCustomFieldMapping(tgtCases, cfID)
	default:
		return nil, fmt.Errorf("unsupported mapping mode: %s", mode)
	}
}

// buildCSVMapping creates mapping from CSV file
func buildCSVMapping(csvPath string) (map[int]int, error) {
	if csvPath == "" {
		return nil, fmt.Errorf("CSV path is required for csv mode")
	}

	file, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV file: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file must have at least a header and one data row")
	}

	// Skip header row
	records = records[1:]

	mapping := make(map[int]int)
	for i, record := range records {
		if len(record) < 2 {
			fmt.Printf("Skipping invalid row %d: insufficient columns\n", i+2)
			continue
		}

		sourceID, err := strconv.Atoi(strings.TrimSpace(record[0]))
		if err != nil {
			fmt.Printf("Skipping invalid row %d: invalid source case ID '%s'\n", i+2, record[0])
			continue
		}

		targetID, err := strconv.Atoi(strings.TrimSpace(record[1]))
		if err != nil {
			fmt.Printf("Skipping invalid row %d: invalid target case ID '%s'\n", i+2, record[1])
			continue
		}

		mapping[sourceID] = targetID
	}

	fmt.Printf("Loaded CSV mapping: %d entries\n", len(mapping))
	return mapping, nil
}

// buildCustomFieldMapping creates mapping from custom field values
func buildCustomFieldMapping(tgtCases map[int]qase.Case, cfID int) (map[int]int, error) {
	if cfID == 0 {
		return nil, fmt.Errorf("custom field ID is required for custom_field mode")
	}

	mapping := make(map[int]int)

	for _, tgtCase := range tgtCases {
		for _, field := range tgtCase.CustomFields {
			if field.ID == cfID {
				sourceID, err := strconv.Atoi(field.Value)
				if err != nil {
					fmt.Printf("Skipping case %d: invalid custom field value '%s'\n", tgtCase.ID, field.Value)
					continue
				}
				mapping[sourceID] = tgtCase.ID
				break
			}
		}
	}

	fmt.Printf("Built custom field mapping: %d entries\n", len(mapping))
	return mapping, nil
}
