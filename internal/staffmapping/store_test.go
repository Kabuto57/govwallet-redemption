package staffmapping_test

import (
	"strings"
	"testing"

	"github.com/govwallet/redemption/internal/staffmapping"
)

func newStoreFromCSV(t *testing.T, csv string) *staffmapping.Store {
	t.Helper()
	s := staffmapping.NewStore()
	if err := s.LoadFromReader(strings.NewReader(csv)); err != nil {
		t.Fatalf("LoadFromReader: %v", err)
	}
	return s
}

const validCSV = `staff_pass_id,team_name,created_at
STAFF_001,TEAM_ALPHA,1700000000000
STAFF_002,TEAM_BETA,1700000001000
`

func TestLookup_Found(t *testing.T) {
	s := newStoreFromCSV(t, validCSV)

	rec, ok := s.Lookup("STAFF_001")
	if !ok {
		t.Fatal("expected STAFF_001 to be found")
	}
	if rec.TeamName != "TEAM_ALPHA" {
		t.Errorf("team = %q, want TEAM_ALPHA", rec.TeamName)
	}
}

func TestLookup_NotFound(t *testing.T) {
	s := newStoreFromCSV(t, validCSV)

	_, ok := s.Lookup("DOES_NOT_EXIST")
	if ok {
		t.Fatal("expected DOES_NOT_EXIST to be absent")
	}
}

func TestLookup_DuplicateIDKeepsLatest(t *testing.T) {
	csv := `staff_pass_id,team_name,created_at
STAFF_DUP,TEAM_OLD,1000
STAFF_DUP,TEAM_NEW,2000
`
	s := newStoreFromCSV(t, csv)

	rec, ok := s.Lookup("STAFF_DUP")
	if !ok {
		t.Fatal("expected STAFF_DUP to be found")
	}
	if rec.TeamName != "TEAM_NEW" {
		t.Errorf("team = %q, want TEAM_NEW (latest created_at wins)", rec.TeamName)
	}
}

func TestLookup_TrimsWhitespace(t *testing.T) {
	s := newStoreFromCSV(t, validCSV)

	rec, ok := s.Lookup("  STAFF_001  ")
	if !ok {
		t.Fatal("expected trimmed lookup to succeed")
	}
	if rec.TeamName != "TEAM_ALPHA" {
		t.Errorf("team = %q, want TEAM_ALPHA", rec.TeamName)
	}
}

func TestLoadFromReader_MissingHeader(t *testing.T) {
	badCSV := `id,team
STAFF_001,TEAM_ALPHA
`
	s := staffmapping.NewStore()
	if err := s.LoadFromReader(strings.NewReader(badCSV)); err == nil {
		t.Fatal("expected error for missing required headers")
	}
}

func TestLoadFromReader_InvalidCreatedAt(t *testing.T) {
	badCSV := `staff_pass_id,team_name,created_at
STAFF_001,TEAM_ALPHA,not-a-number
`
	s := staffmapping.NewStore()
	if err := s.LoadFromReader(strings.NewReader(badCSV)); err == nil {
		t.Fatal("expected error for non-numeric created_at")
	}
}
