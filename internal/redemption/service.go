package redemption

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Record represents a single redemption event.
type Record struct {
	TeamName    string
	RedeemedAt  int64 // epoch milliseconds
}

// Service manages redemption state and enforces the one-redemption-per-team rule.
// It is safe for concurrent use.
type Service struct {
	mu          sync.RWMutex
	redemptions map[string]Record // keyed by team_name (normalised to upper-case)
	filePath    string            // path used for persistence (may be empty)
}

// NewService creates a Service with an optional CSV backing file.
// If filePath is non-empty, existing redemption records are loaded from it and
// new redemptions are appended to it.
func NewService(filePath string) (*Service, error) {
	svc := &Service{
		redemptions: make(map[string]Record),
		filePath:    filePath,
	}
	if filePath != "" {
		if err := svc.loadFromFile(filePath); err != nil {
			return nil, err
		}
	}
	return svc, nil
}

// CanRedeem returns true when the given team has not yet redeemed their gift.
func (s *Service) CanRedeem(teamName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, alreadyRedeemed := s.redemptions[normalise(teamName)]
	return !alreadyRedeemed
}

// Redeem records a redemption for teamName if the team has not redeemed yet.
// It returns (record, true) on success, or (zero, false) if already redeemed.
func (s *Service) Redeem(teamName string) (Record, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := normalise(teamName)
	if _, exists := s.redemptions[key]; exists {
		return Record{}, false
	}

	rec := Record{
		TeamName:   teamName,
		RedeemedAt: time.Now().UnixMilli(),
	}
	s.redemptions[key] = rec

	if s.filePath != "" {
		// Best-effort append; log the error but don't fail the in-memory state.
		if err := s.appendToFile(rec); err != nil {
			fmt.Printf("warning: could not persist redemption for %q: %v\n", teamName, err)
		}
	}

	return rec, true
}

// GetRedemption returns the redemption record for a team, if one exists.
func (s *Service) GetRedemption(teamName string) (Record, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.redemptions[normalise(teamName)]
	return rec, ok
}

// AllRedemptions returns a snapshot of all recorded redemptions.
func (s *Service) AllRedemptions() []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Record, 0, len(s.redemptions))
	for _, r := range s.redemptions {
		out = append(out, r)
	}
	return out
}

// --- persistence ------------------------------------------------------------

// loadFromFile reads an existing redemption CSV into memory.
// A missing file is treated as an empty store (not an error).
func (s *Service) loadFromFile(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil // first run; file will be created on first redemption
	}
	if err != nil {
		return fmt.Errorf("open redemption file: %w", err)
	}
	defer f.Close()

	return s.loadFromReader(f)
}

func (s *Service) loadFromReader(r io.Reader) error {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err == io.EOF {
		return nil // empty file
	}
	if err != nil {
		return fmt.Errorf("read redemption CSV header: %w", err)
	}
	if err := validateHeaders(headers); err != nil {
		return err
	}

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read redemption CSV row: %w", err)
		}
		rec, err := parseRow(row)
		if err != nil {
			return fmt.Errorf("parse redemption row: %w", err)
		}
		s.redemptions[normalise(rec.TeamName)] = rec
	}
	return nil
}

// appendToFile adds a single redemption record to the backing CSV.
// It creates the file with headers if it does not yet exist.
func (s *Service) appendToFile(rec Record) error {
	needsHeader := false
	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		needsHeader = true
	}

	f, err := os.OpenFile(s.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if needsHeader {
		if err := w.Write([]string{"team_name", "redeemed_at"}); err != nil {
			return err
		}
	}
	if err := w.Write([]string{rec.TeamName, strconv.FormatInt(rec.RedeemedAt, 10)}); err != nil {
		return err
	}
	w.Flush()
	return w.Error()
}

// --- helpers ----------------------------------------------------------------

func normalise(teamName string) string {
	return strings.ToUpper(strings.TrimSpace(teamName))
}

func validateHeaders(headers []string) error {
	required := map[string]bool{"team_name": false, "redeemed_at": false}
	for _, h := range headers {
		key := strings.ToLower(strings.TrimSpace(h))
		if _, ok := required[key]; ok {
			required[key] = true
		}
	}
	for h, found := range required {
		if !found {
			return fmt.Errorf("redemption CSV missing required header %q", h)
		}
	}
	return nil
}

func parseRow(row []string) (Record, error) {
	if len(row) < 2 {
		return Record{}, fmt.Errorf("expected at least 2 fields, got %d", len(row))
	}
	redeemedAt, err := strconv.ParseInt(strings.TrimSpace(row[1]), 10, 64)
	if err != nil {
		return Record{}, fmt.Errorf("invalid redeemed_at %q: %w", row[1], err)
	}
	return Record{
		TeamName:   strings.TrimSpace(row[0]),
		RedeemedAt: redeemedAt,
	}, nil
}
