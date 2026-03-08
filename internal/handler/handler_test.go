package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/govwallet/redemption/internal/handler"
	"github.com/govwallet/redemption/internal/redemption"
	"github.com/govwallet/redemption/internal/staffmapping"
)

const sampleCSV = `staff_pass_id,team_name,created_at
STAFF_001,TEAM_ALPHA,1700000000000
STAFF_002,TEAM_BETA,1700000001000
`

func setup(t *testing.T) http.Handler {
	t.Helper()

	store := staffmapping.NewStore()
	if err := store.LoadFromReader(strings.NewReader(sampleCSV)); err != nil {
		t.Fatalf("load staff mapping: %v", err)
	}

	svc, err := redemption.NewService("")
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	mux := http.NewServeMux()
	handler.New(mux, store, svc)
	return mux
}

func postJSON(t *testing.T, h http.Handler, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func getJSON(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestLookupStaff_Found(t *testing.T) {
	h := setup(t)
	rr := postJSON(t, h, "/staff/lookup", map[string]string{"staff_pass_id": "STAFF_001"})

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body)
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["team_name"] != "TEAM_ALPHA" {
		t.Errorf("team_name = %v, want TEAM_ALPHA", resp["team_name"])
	}
}

func TestLookupStaff_NotFound(t *testing.T) {
	h := setup(t)
	rr := postJSON(t, h, "/staff/lookup", map[string]string{"staff_pass_id": "UNKNOWN"})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestLookupStaff_MissingBody(t *testing.T) {
	h := setup(t)
	rr := postJSON(t, h, "/staff/lookup", map[string]string{"staff_pass_id": ""})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestCheckRedemption_Eligible(t *testing.T) {
	h := setup(t)
	rr := postJSON(t, h, "/redemption/check", map[string]string{"team_name": "TEAM_ALPHA"})

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["can_redeem"] != true {
		t.Errorf("can_redeem = %v, want true", resp["can_redeem"])
	}
}

func TestCheckRedemption_AlreadyRedeemed(t *testing.T) {
	h := setup(t)
	postJSON(t, h, "/redemption/redeem", map[string]string{"staff_pass_id": "STAFF_001"})
	rr := postJSON(t, h, "/redemption/check", map[string]string{"team_name": "TEAM_ALPHA"})

	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["can_redeem"] != false {
		t.Errorf("can_redeem = %v, want false after redemption", resp["can_redeem"])
	}
}

func TestRedeem_Success(t *testing.T) {
	h := setup(t)
	rr := postJSON(t, h, "/redemption/redeem", map[string]string{"staff_pass_id": "STAFF_001"})

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rr.Code, rr.Body)
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["team_name"] != "TEAM_ALPHA" {
		t.Errorf("team_name = %v, want TEAM_ALPHA", resp["team_name"])
	}
}

func TestRedeem_UnknownStaff(t *testing.T) {
	h := setup(t)
	rr := postJSON(t, h, "/redemption/redeem", map[string]string{"staff_pass_id": "UNKNOWN"})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestRedeem_DuplicateReturnsConflict(t *testing.T) {
	h := setup(t)
	postJSON(t, h, "/redemption/redeem", map[string]string{"staff_pass_id": "STAFF_001"})
	rr := postJSON(t, h, "/redemption/redeem", map[string]string{"staff_pass_id": "STAFF_001"})

	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rr.Code)
	}
}

func TestRedeem_DifferentTeamSucceeds(t *testing.T) {
	h := setup(t)
	postJSON(t, h, "/redemption/redeem", map[string]string{"staff_pass_id": "STAFF_001"})

	rr2 := postJSON(t, h, "/redemption/redeem", map[string]string{"staff_pass_id": "STAFF_002"})
	if rr2.Code != http.StatusOK {
		t.Fatalf("STAFF_002 (TEAM_BETA) should succeed; status = %d, body: %s", rr2.Code, rr2.Body)
	}
}

func TestListRedemptions(t *testing.T) {
	h := setup(t)
	postJSON(t, h, "/redemption/redeem", map[string]string{"staff_pass_id": "STAFF_001"})
	postJSON(t, h, "/redemption/redeem", map[string]string{"staff_pass_id": "STAFF_002"})

	rr := getJSON(t, h, "/redemption/list")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp map[string]any
	json.NewDecoder(rr.Body).Decode(&resp)
	list, ok := resp["redemptions"].([]any)
	if !ok {
		t.Fatalf("expected redemptions array; got %T", resp["redemptions"])
	}
	if len(list) != 2 {
		t.Errorf("redemptions count = %d, want 2", len(list))
	}
}

func TestHealth(t *testing.T) {
	h := setup(t)
	rr := getJSON(t, h, "/health")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
}
