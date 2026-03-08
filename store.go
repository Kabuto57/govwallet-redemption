package staffmapping

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// StaffRecord represents a single row in the staff mapping CSV.
type StaffRecord struct {
	StaffPassID string
	TeamName    string
	CreatedAt   int64 // epoch milliseconds
}

// Store holds an in-memory lookup of staff pass IDs to their team records.
type Store struct {
	records map[string]StaffRecord // keyed by staff_pass_id
}

// NewStore returns an empty Store.
func NewStore() *Store {
	return &Store{records: make(map[string]StaffRecord)}
}

// LoadFromFile reads the CSV at the given path and populates the Store.
// Duplicate staff_pass_id entries are resolved by keeping the record with
// the latest created_at timestamp, matching typical "last-write-wins" semantics.
func (s *Store) LoadFromFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open staff mapping file: %w", err)
	}
	defer f.Close()

	return s.LoadFromReader(f)
}

// LoadFromReader parses CSV data from the provided reader.
// Exposed separately so unit tests can pass an in-memory reader.
func (s *Store) LoadFromReader(r io.Reader) error {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		return fmt.Errorf("read CSV header: %w", err)
	}
	if err := validateHeaders(headers, []string{"staff_pass_id", "team_name", "created_at"}); err != nil {
		return err
	}

	lineNum := 1
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read CSV row %d: %w", lineNum, err)
		}
		lineNum++

		rec, err := parseRow(row)
		if err != nil {
			return fmt.Errorf("parse row %d: %w", lineNum, err)
		}

		// Keep the record with the latest created_at on duplicate IDs.
		if existing, ok := s.records[rec.StaffPassID]; !ok || rec.CreatedAt > existing.CreatedAt {
			s.records[rec.StaffPassID] = rec
		}
	}

	return nil
}

// Lookup returns the StaffRecord for the given staff pass ID.
// The second return value is false when the ID is not found.
func (s *Store) Lookup(staffPassID string) (StaffRecord, bool) {
	rec, ok := s.records[strings.TrimSpace(staffPassID)]
	return rec, ok
}

// --- helpers ----------------------------------------------------------------

func validateHeaders(got, want []string) error {
	if len(got) < len(want) {
		return fmt.Errorf("CSV missing required headers; got %v, want %v", got, want)
	}
	gotNorm := make(map[string]struct{}, len(got))
	for _, h := range got {
		gotNorm[strings.ToLower(strings.TrimSpace(h))] = struct{}{}
	}
	for _, w := range want {
		if _, ok := gotNorm[w]; !ok {
			return fmt.Errorf("CSV missing required header %q", w)
		}
	}
	return nil
}

func parseRow(row []string) (StaffRecord, error) {
	if len(row) < 3 {
		return StaffRecord{}, fmt.Errorf("expected at least 3 fields, got %d", len(row))
	}
	createdAt, err := strconv.ParseInt(strings.TrimSpace(row[2]), 10, 64)
	if err != nil {
		return StaffRecord{}, fmt.Errorf("invalid created_at %q: %w", row[2], err)
	}
	return StaffRecord{
		StaffPassID: strings.TrimSpace(row[0]),
		TeamName:    strings.TrimSpace(row[1]),
		CreatedAt:   createdAt,
	}, nil
}
