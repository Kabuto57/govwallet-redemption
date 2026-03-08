package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/govwallet/redemption/internal/redemption"
	"github.com/govwallet/redemption/internal/staffmapping"
)

// Handler wires together the HTTP routes and the domain services.
type Handler struct {
	staffStore      *staffmapping.Store
	redemptionSvc   *redemption.Service
}

// New creates a Handler and registers routes on mux.
// Pass http.DefaultServeMux or any *http.ServeMux.
func New(mux *http.ServeMux, staffStore *staffmapping.Store, redemptionSvc *redemption.Service) *Handler {
	h := &Handler{
		staffStore:    staffStore,
		redemptionSvc: redemptionSvc,
	}
	mux.HandleFunc("/staff/lookup", h.lookupStaff)
	mux.HandleFunc("/redemption/check", h.checkRedemption)
	mux.HandleFunc("/redemption/redeem", h.redeem)
	mux.HandleFunc("/redemption/list", h.listRedemptions)
	mux.HandleFunc("/health", h.health)
	return h
}

// POST /staff/lookup
// Body: { "staff_pass_id": "STAFF_001" }
// Response 200: { "staff_pass_id": "STAFF_001", "team_name": "TEAM_ALPHA", "created_at": 1700000000000 }
// Response 404: { "error": "staff pass ID not found" }
func (h *Handler) lookupStaff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		StaffPassID string `json:"staff_pass_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.StaffPassID) == "" {
		writeError(w, http.StatusBadRequest, "staff_pass_id is required")
		return
	}

	rec, ok := h.staffStore.Lookup(req.StaffPassID)
	if !ok {
		writeError(w, http.StatusNotFound, "staff pass ID not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"staff_pass_id": rec.StaffPassID,
		"team_name":     rec.TeamName,
		"created_at":    rec.CreatedAt,
	})
}

// POST /redemption/check
// Body: { "team_name": "TEAM_ALPHA" }
// Response 200: { "team_name": "TEAM_ALPHA", "can_redeem": true }
func (h *Handler) checkRedemption(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		TeamName string `json:"team_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.TeamName) == "" {
		writeError(w, http.StatusBadRequest, "team_name is required")
		return
	}

	canRedeem := h.redemptionSvc.CanRedeem(req.TeamName)
	resp := map[string]any{
		"team_name":  req.TeamName,
		"can_redeem": canRedeem,
	}
	if !canRedeem {
		if existing, ok := h.redemptionSvc.GetRedemption(req.TeamName); ok {
			resp["redeemed_at"] = existing.RedeemedAt
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// POST /redemption/redeem
// Body: { "staff_pass_id": "STAFF_001" }
// Response 200: { "team_name": "TEAM_ALPHA", "redeemed_at": 1700100000000 }
// Response 404: { "error": "staff pass ID not found" }
// Response 409: { "error": "team has already redeemed their gift", "redeemed_at": ... }
func (h *Handler) redeem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		StaffPassID string `json:"staff_pass_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.StaffPassID) == "" {
		writeError(w, http.StatusBadRequest, "staff_pass_id is required")
		return
	}

	// Step 1: Identify the representative's team.
	staffRec, ok := h.staffStore.Lookup(req.StaffPassID)
	if !ok {
		writeError(w, http.StatusNotFound, "staff pass ID not found")
		return
	}

	// Step 2 & 3: Attempt redemption; reject if already redeemed.
	record, redeemed := h.redemptionSvc.Redeem(staffRec.TeamName)
	if !redeemed {
		existing, _ := h.redemptionSvc.GetRedemption(staffRec.TeamName)
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":       "team has already redeemed their gift",
			"team_name":   staffRec.TeamName,
			"redeemed_at": existing.RedeemedAt,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"team_name":   record.TeamName,
		"redeemed_at": record.RedeemedAt,
	})
}

// GET /redemption/list
func (h *Handler) listRedemptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	records := h.redemptionSvc.AllRedemptions()
	type row struct {
		TeamName   string `json:"team_name"`
		RedeemedAt int64  `json:"redeemed_at"`
	}
	out := make([]row, 0, len(records))
	for _, rec := range records {
		out = append(out, row{TeamName: rec.TeamName, RedeemedAt: rec.RedeemedAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"redemptions": out})
}

// GET /health
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- helpers ----------------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
