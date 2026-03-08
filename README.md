# GovWallet Gift Redemption System

A Go HTTP service that manages Christmas gift redemptions for department teams. Each team may send any representative (identified by their staff pass) to redeem their team's gift — but each team can only redeem once.

---

## Architecture Overview

```
cmd/server/          – main entrypoint; wires dependencies and starts HTTP server
internal/
  staffmapping/      – loads & queries the staff_pass_id → team_name CSV mapping
  redemption/        – enforces one-redemption-per-team; persists state to CSV
  handler/           – HTTP handlers that compose the two domain services
data/
  staff_mapping.csv  – input mapping (read-only at runtime)
  redemptions.csv    – redemption ledger (created/appended at runtime)
```

### Design Decisions

| Concern | Decision | Rationale |
|---|---|---|
| **Redemption storage** | In-memory `map` backed by a CSV file | Simple, zero-dependency, survives restarts. A relational DB would be appropriate for production scale but is overkill for this scope. |
| **Concurrency** | `sync.RWMutex` on the `Service` | Allows concurrent reads; serialises writes so exactly one goroutine wins a race. |
| **Duplicate staff IDs in mapping** | Last `created_at` wins | Mirrors a "latest-record-wins" data pipeline convention. |
| **Team name matching** | Upper-cased and trimmed | Prevents `team_alpha` / `TEAM_ALPHA` from being treated as distinct. |
| **No framework** | `net/http` only | Keeps the binary small and dependencies at zero; the routing surface is tiny. |

---

## Prerequisites

- **Go 1.21+** – [install](https://go.dev/dl/)  
  *or* **Docker** (no Go installation required)

---

## Running Locally (without Docker)

```bash
# Clone the repository
git clone <your-repo-url>
cd govwallet-redemption

# Run all tests
make test

# Build and start the server (default: :8080)
make run

# Override defaults via flags
./bin/redemption-server \
  -addr :9090 \
  -staff-mapping /path/to/staff_mapping.csv \
  -redemption-data /path/to/redemptions.csv
```

Environment variables are also supported as an alternative to flags:

| Variable | Default |
|---|---|
| `ADDR` | `:8080` |
| `STAFF_MAPPING_CSV` | `data/staff_mapping.csv` |
| `REDEMPTION_CSV` | `data/redemptions.csv` |

---

## Running with Docker

```bash
make docker-build
make docker-run          # maps container :8080 → host :8080

# To persist redemption data on the host:
docker run --rm -p 8080:8080 \
  -v "$(pwd)/data:/app/data" \
  govwallet-redemption
```

---

## API Reference

All request/response bodies are JSON.

### `POST /staff/lookup`

Look up a staff pass ID and return the associated team.

**Request**
```json
{ "staff_pass_id": "STAFF_001" }
```

**Response 200**
```json
{ "staff_pass_id": "STAFF_001", "team_name": "TEAM_ALPHA", "created_at": 1700000000000 }
```

**Response 404** – ID not found
```json
{ "error": "staff pass ID not found" }
```

---

### `POST /redemption/check`

Check whether a team is still eligible to redeem.

**Request**
```json
{ "team_name": "TEAM_ALPHA" }
```

**Response 200 – eligible**
```json
{ "team_name": "TEAM_ALPHA", "can_redeem": true }
```

**Response 200 – already redeemed**
```json
{ "team_name": "TEAM_ALPHA", "can_redeem": false, "redeemed_at": 1700100000000 }
```

---

### `POST /redemption/redeem`

Attempt a redemption on behalf of the team identified by a staff pass.  
Steps performed by the server:
1. Resolve `staff_pass_id` → `team_name`.
2. Check whether that team has already redeemed.
3. Record the redemption (or reject if already done).

**Request**
```json
{ "staff_pass_id": "STAFF_001" }
```

**Response 200 – success**
```json
{ "team_name": "TEAM_ALPHA", "redeemed_at": 1700100000000 }
```

**Response 404** – unknown staff pass
```json
{ "error": "staff pass ID not found" }
```

**Response 409** – team already redeemed
```json
{ "error": "team has already redeemed their gift", "team_name": "TEAM_ALPHA", "redeemed_at": 1700100000000 }
```

---

### `GET /redemption/list`

Return all recorded redemptions.

**Response 200**
```json
{
  "redemptions": [
    { "team_name": "TEAM_ALPHA", "redeemed_at": 1700100000000 },
    { "team_name": "TEAM_BETA",  "redeemed_at": 1700100001000 }
  ]
}
```

---

### `GET /health`

Liveness check.

**Response 200**
```json
{ "status": "ok" }
```

---

## Quick Smoke Test (curl)

```bash
# Lookup a staff member
curl -s -X POST http://localhost:8080/staff/lookup \
  -H 'Content-Type: application/json' \
  -d '{"staff_pass_id":"STAFF_001"}' | jq .

# Check if team can redeem
curl -s -X POST http://localhost:8080/redemption/check \
  -H 'Content-Type: application/json' \
  -d '{"team_name":"TEAM_ALPHA"}' | jq .

# Redeem gift
curl -s -X POST http://localhost:8080/redemption/redeem \
  -H 'Content-Type: application/json' \
  -d '{"staff_pass_id":"STAFF_001"}' | jq .

# Try to redeem again (expect 409)
curl -s -X POST http://localhost:8080/redemption/redeem \
  -H 'Content-Type: application/json' \
  -d '{"staff_pass_id":"STAFF_001"}' | jq .

# List all redemptions
curl -s http://localhost:8080/redemption/list | jq .
```

---

## Assumptions & Interpretations

1. **One redemption per team, not per staff member.** Any representative from a team may trigger the redemption, and once done the entire team is marked as redeemed.
2. **Staff pass IDs are globally unique.** A staff member belongs to exactly one team; the mapping enforces this.
3. **Duplicate rows in the CSV** (same `staff_pass_id`, different `team_name`) are resolved by keeping the record with the highest `created_at` timestamp.
4. **Redemption data persists across restarts** via a CSV append log. Concurrent writes are safe due to the in-process mutex; a full database would be required for multi-instance deployments.
5. **No authentication** is implemented (out of scope for this assessment).
