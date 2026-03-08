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

| Concern                            | Decision                             | Rationale                                                                                                                             |
| ---------------------------------- | ------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------- |
| **Redemption storage**             | In-memory `map` backed by a CSV file | Simple, zero-dependency, survives restarts. A relational DB would be appropriate for production scale but is overkill for this scope. |
| **Concurrency**                    | `sync.RWMutex` on the `Service`      | Allows concurrent reads; serialises writes so exactly one goroutine wins a race.                                                      |
| **Duplicate staff IDs in mapping** | Last `created_at` wins               | Mirrors a "latest-record-wins" data pipeline convention.                                                                              |
| **Team name matching**             | Upper-cased and trimmed              | Prevents `team_alpha` / `TEAM_ALPHA` from being treated as distinct.                                                                  |
| **No framework**                   | `net/http` only                      | Keeps the binary small and dependencies at zero; the routing surface is tiny.                                                         |

---

## Prerequisites

- **Go 1.23+** – [install](https://go.dev/dl/)
- **Docker Desktop** – [install](https://www.docker.com/products/docker-desktop) _(optional)_

---

## Running Locally (without Docker)

```bash
# Clone the repository
git clone <your-repo-url>
cd govwallet-redemption

# Run all unit tests
go test ./... -v

# Start the server (default: :8080)
go run ./cmd/server
```

---

## Running with Docker

```bash
# Build the image
docker build -t govwallet-redemption .

# Run the container
docker run --rm -p 8080:8080 -v $(pwd)/data:/app/data govwallet-redemption
```

You should see:

```
loaded staff mapping from "data/staff_mapping.csv"
redemption data backed by "data/redemptions.csv"
server listening on :8080
```

---

## Unit Tests

Run all unit tests with:

```bash
go test ./... -v
```

Expected output:

```
=== RUN   TestLookupStaff_Found
--- PASS: TestLookupStaff_Found (0.00s)
=== RUN   TestLookupStaff_NotFound
--- PASS: TestLookupStaff_NotFound (0.00s)
=== RUN   TestLookupStaff_MissingBody
--- PASS: TestLookupStaff_MissingBody (0.00s)
=== RUN   TestCheckRedemption_Eligible
--- PASS: TestCheckRedemption_Eligible (0.00s)
=== RUN   TestCheckRedemption_AlreadyRedeemed
--- PASS: TestCheckRedemption_AlreadyRedeemed (0.00s)
=== RUN   TestRedeem_Success
--- PASS: TestRedeem_Success (0.00s)
=== RUN   TestRedeem_UnknownStaff
--- PASS: TestRedeem_UnknownStaff (0.00s)
=== RUN   TestRedeem_DuplicateReturnsConflict
--- PASS: TestRedeem_DuplicateReturnsConflict (0.00s)
=== RUN   TestRedeem_DifferentTeamSucceeds
--- PASS: TestRedeem_DifferentTeamSucceeds (0.00s)
=== RUN   TestListRedemptions
--- PASS: TestListRedemptions (0.00s)
=== RUN   TestHealth
--- PASS: TestHealth (0.00s)
ok  github.com/govwallet/redemption/internal/handler
ok  github.com/govwallet/redemption/internal/redemption
ok  github.com/govwallet/redemption/internal/staffmapping
```

---

## API Tests (Manual)

Make sure the server is running first, then reset any existing redemption data:

```bash
rm -f data/redemptions.csv
```

Then run all API tests in one command:

```bash
echo "TEST 1: Look up real staff member" && \
curl -s -X POST http://localhost:8080/staff/lookup \
  -H "Content-Type: application/json" \
  -d '{"staff_pass_id":"STAFF_P001"}' && \
echo "" && \
echo "TEST 2: Look up fake staff member" && \
curl -s -X POST http://localhost:8080/staff/lookup \
  -H "Content-Type: application/json" \
  -d '{"staff_pass_id":"FAKE_999"}' && \
echo "" && \
echo "TEST 3: Check if team can redeem" && \
curl -s -X POST http://localhost:8080/redemption/check \
  -H "Content-Type: application/json" \
  -d '{"team_name":"TEAM_ALPHA"}' && \
echo "" && \
echo "TEST 4: Redeem a gift" && \
curl -s -X POST http://localhost:8080/redemption/redeem \
  -H "Content-Type: application/json" \
  -d '{"staff_pass_id":"STAFF_P001"}' && \
echo "" && \
echo "TEST 5: Same team tries again - should be blocked" && \
curl -s -X POST http://localhost:8080/redemption/redeem \
  -H "Content-Type: application/json" \
  -d '{"staff_pass_id":"STAFF_P002"}' && \
echo "" && \
echo "TEST 6: Different team redeems - should succeed" && \
curl -s -X POST http://localhost:8080/redemption/redeem \
  -H "Content-Type: application/json" \
  -d '{"staff_pass_id":"STAFF_P004"}' && \
echo "" && \
echo "TEST 7: List all redemptions" && \
curl -s http://localhost:8080/redemption/list && \
echo ""
```

### Expected Results

| Test   | What it checks               | Expected response                                                                |
| ------ | ---------------------------- | -------------------------------------------------------------------------------- |
| TEST 1 | Valid staff lookup           | `{"staff_pass_id":"STAFF_P001","team_name":"TEAM_ALPHA",...}`                    |
| TEST 2 | Invalid staff lookup         | `{"error":"staff pass ID not found"}`                                            |
| TEST 3 | Team eligible to redeem      | `{"can_redeem":true,"team_name":"TEAM_ALPHA"}`                                   |
| TEST 4 | Successful redemption        | `{"team_name":"TEAM_ALPHA","redeemed_at":...}`                                   |
| TEST 5 | Duplicate redemption blocked | `{"error":"team has already redeemed their gift",...}`                           |
| TEST 6 | Different team can redeem    | `{"team_name":"TEAM_BETA","redeemed_at":...}`                                    |
| TEST 7 | List all redemptions         | `{"redemptions":[{"team_name":"TEAM_ALPHA",...},{"team_name":"TEAM_BETA",...}]}` |

---

## API Reference

All request/response bodies are JSON.

### `POST /staff/lookup`

Look up a staff pass ID and return the associated team.

**Request**

```json
{ "staff_pass_id": "STAFF_P001" }
```

**Response 200**

```json
{
  "staff_pass_id": "STAFF_P001",
  "team_name": "TEAM_ALPHA",
  "created_at": 1700000000000
}
```

**Response 404**

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

**Request**

```json
{ "staff_pass_id": "STAFF_P001" }
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
{
  "error": "team has already redeemed their gift",
  "team_name": "TEAM_ALPHA",
  "redeemed_at": 1700100000000
}
```

---

### `GET /redemption/list`

Return all recorded redemptions.

**Response 200**

```json
{
  "redemptions": [
    { "team_name": "TEAM_ALPHA", "redeemed_at": 1700100000000 },
    { "team_name": "TEAM_BETA", "redeemed_at": 1700100001000 }
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

## Assumptions & Interpretations

1. **One redemption per team, not per staff member.** Any representative from a team may trigger the redemption, and once done the entire team is marked as redeemed.
2. **Staff pass IDs are globally unique.** A staff member belongs to exactly one team; the mapping enforces this.
3. **Duplicate rows in the CSV** (same `staff_pass_id`, different `team_name`) are resolved by keeping the record with the highest `created_at` timestamp.
4. **Redemption data persists across restarts** via a CSV append log. Concurrent writes are safe due to the in-process mutex; a full database would be required for multi-instance deployments.
5. **No authentication** is implemented (out of scope for this assessment).
