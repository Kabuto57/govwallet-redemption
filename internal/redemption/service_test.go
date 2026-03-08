package redemption_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/govwallet/redemption/internal/redemption"
)

func newService(t *testing.T) *redemption.Service {
	t.Helper()
	svc, err := redemption.NewService("")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

func TestCanRedeem_InitiallyTrue(t *testing.T) {
	svc := newService(t)
	if !svc.CanRedeem("TEAM_ALPHA") {
		t.Error("expected TEAM_ALPHA to be eligible before any redemption")
	}
}

func TestRedeem_Success(t *testing.T) {
	svc := newService(t)
	rec, ok := svc.Redeem("TEAM_ALPHA")
	if !ok {
		t.Fatal("expected first redemption to succeed")
	}
	if rec.TeamName != "TEAM_ALPHA" {
		t.Errorf("TeamName = %q, want TEAM_ALPHA", rec.TeamName)
	}
	if rec.RedeemedAt <= 0 {
		t.Error("expected RedeemedAt to be a positive epoch millisecond timestamp")
	}
}

func TestRedeem_DuplicateFails(t *testing.T) {
	svc := newService(t)
	if _, ok := svc.Redeem("TEAM_ALPHA"); !ok {
		t.Fatal("first redemption must succeed")
	}
	if _, ok := svc.Redeem("TEAM_ALPHA"); ok {
		t.Error("expected second redemption for same team to be rejected")
	}
}

func TestCanRedeem_FalseAfterRedemption(t *testing.T) {
	svc := newService(t)
	svc.Redeem("TEAM_ALPHA")
	if svc.CanRedeem("TEAM_ALPHA") {
		t.Error("expected CanRedeem to return false after redemption")
	}
}

func TestRedeem_CaseInsensitiveTeamName(t *testing.T) {
	svc := newService(t)
	if _, ok := svc.Redeem("team_alpha"); !ok {
		t.Fatal("first redemption must succeed")
	}
	if _, ok := svc.Redeem("TEAM_ALPHA"); ok {
		t.Error("expected case-insensitive duplicate to be rejected")
	}
}

func TestGetRedemption_Found(t *testing.T) {
	svc := newService(t)
	svc.Redeem("TEAM_BETA")

	rec, ok := svc.GetRedemption("TEAM_BETA")
	if !ok {
		t.Fatal("expected redemption record to be retrievable")
	}
	if rec.TeamName != "TEAM_BETA" {
		t.Errorf("TeamName = %q, want TEAM_BETA", rec.TeamName)
	}
}

func TestGetRedemption_NotFound(t *testing.T) {
	svc := newService(t)
	_, ok := svc.GetRedemption("TEAM_UNKNOWN")
	if ok {
		t.Error("expected no record for unredeemed team")
	}
}

func TestAllRedemptions(t *testing.T) {
	svc := newService(t)
	teams := []string{"TEAM_A", "TEAM_B", "TEAM_C"}
	for _, team := range teams {
		svc.Redeem(team)
	}

	all := svc.AllRedemptions()
	if len(all) != len(teams) {
		t.Errorf("AllRedemptions returned %d records, want %d", len(all), len(teams))
	}
}

func TestRedeem_ConcurrentSafety(t *testing.T) {
	svc := newService(t)

	const workers = 50
	results := make(chan bool, workers)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok := svc.Redeem("SHARED_TEAM")
			results <- ok
		}()
	}

	wg.Wait()
	close(results)

	successCount := 0
	for ok := range results {
		if ok {
			successCount++
		}
	}

	if successCount != 1 {
		t.Errorf("expected exactly 1 successful concurrent redemption, got %d", successCount)
	}
}

func TestLoadFromReader_ExistingRedemptions(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/redemptions.csv"

	svc1, err := redemption.NewService(path)
	if err != nil {
		t.Fatalf("NewService (1st): %v", err)
	}
	if _, ok := svc1.Redeem("TEAM_PERSIST"); !ok {
		t.Fatal("redemption should succeed on first service")
	}

	svc2, err := redemption.NewService(path)
	if err != nil {
		t.Fatalf("NewService (2nd): %v", err)
	}
	if svc2.CanRedeem("TEAM_PERSIST") {
		t.Error("second service should see the persisted redemption and deny CanRedeem")
	}
}

var _ = strings.TrimSpace
