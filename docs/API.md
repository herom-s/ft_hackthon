# ft_hackthon API Specification

## Overview

The ft_hackthon API provides REST endpoints for authentication, project submission, and grading status management.

## Base URL

```
http://localhost:8000/api/v1
```

## Authentication

The following endpoints require authentication via Bearer token:
`/grade/submit`, `/grade/status/`, `/grade/jobs`, `/grade/repo`, `/user/me`.

Public endpoints (no auth required): `/health`, `/grade/suites`, `/grade/leaderboard/`, `/grade/plagiarism/`.

```
Authorization: Bearer <auth_token>
```

## Endpoints

### WebSocket

| Path | Auth | Description |
|------|------|-------------|
| `/ws/grade/status/{job_id}` | Token (query) | Real-time job status updates |
| `/ws/grade/jobs` | Token (query) | Real-time jobs list |

### Authentication

#### POST /auth/login

Authenticate user and receive auth token.

**Request:**
```json
{
  "username": "student",
  "password": "securepassword"
}
```

**Response (200):**
```json
{
  "token": "a1b2c3d4e5f6...64-char-hex",
  "user": "student",
  "gitea_clone_url": "http://gitea:3000/student/repo.git",
  "gitea_token": "gitea-access-token"
}
```

**Response (401):**
```json
{
  "error": "Invalid credentials"
}
```

#### POST /auth/register

Create a new user account.

**Request:**
```json
{
  "username": "student",
  "password": "securepassword"
}
```

**Response (201):**
```json
{
  "token": "a1b2c3d4e5f6...64-char-hex",
  "user": "student",
  "gitea_clone_url": "http://gitea:3000/student/repo.git",
  "gitea_token": "gitea-access-token"
}
```

**Response (400):**
```json
{
  "error": "Username already exists"
}
```

### Grading

#### POST /grade/submit

Submit a project for grading.

**Headers:**
```
Authorization: Bearer <auth_token>
Content-Type: application/json
```

**Request:**
```json
{
  "commit_sha": "abc123def456ghi789jkl012mnopqr345stu678",
  "suite": "libft-tester"
}
```

The `suite` field is optional — if omitted, the grader falls back to auto-detection based on detect files in the workspace.

**Response (202 Accepted):**
```json
{
  "job_id": "job-uuid-1234-5678",
  "status": "queued"
}
```

**Response (400):**
```json
{
  "error": "Invalid commit SHA format"
}
```

#### GET /grade/status/{job_id}

Get the status of a grading job.

**Headers:**
```
Authorization: Bearer <auth_token>
```

**Response (200):**
```json
{
  "job_id": "job-uuid-1234-5678",
  "status": "processing",
  "message": "Running parser tests...",
  "result": null
}
```

**On Completion (200):**
```json
{
  "job_id": "job-uuid-1234-5678",
  "status": "completed",
  "message": "Grading completed",
  "result": {
    "parser_success": true,
    "benchmark_ms": 150,
    "final_score": 95,
    "details": "All tests passed. Excellent performance!",
    "code_checksum": "sha256-of-submitted-code",
    "challenges": [
      {
        "name": "ch1",
        "title": "Challenge 1",
        "passed": true,
        "points": 10,
        "tests_run": 5,
        "tests_passed": 5,
        "benchmark_ms": 20
      }
    ]
  }
}
```

**Response (404):**
```json
{
  "error": "Job not found"
}
```

#### GET /grade/jobs

List all grading jobs for the authenticated user.

**Headers:**
```
Authorization: Bearer <auth_token>
```

**Response (200):**
```json
{
  "jobs": [
    {
      "id": "job-uuid-1234",
      "status": "completed",
      "suite": "libft",
      "message": "Grading completed",
      "created_at": "2024-06-03T10:30:00Z"
    }
  ]
}
```

#### POST /grade/repo

Link a git repository for the authenticated user.

**Headers:**
```
Authorization: Bearer <auth_token>
Content-Type: application/json
```

**Request:**
```json
{
  "gitea_clone_url": "http://gitea:3000/student/repo.git",
  "gitea_token": "gitea-access-token"
}
```

**Response (200):**
```json
{
  "message": "Repository linked successfully"
}
```

#### GET /grade/suites/{suite}/challenges

List challenges and subjects for a specific test suite.

**Response (200):**
```json
{
  "challenges": [
    {
      "name": "ch1",
      "title": "String Length",
      "points": 10,
      "subject": "Implement ft_strlen that returns the length of a string"
    }
  ]
}
```

#### GET /grade/plagiarism/{hackathon}

Check for duplicate submissions (same code checksum across different users).

**Response (200):**
```json
{
  "groups": [
    {
      "checksum": "sha256-hash",
      "user_count": 2,
      "users": ["alice", "bob"],
      "job_ids": ["job-uuid-1", "job-uuid-2"]
    }
  ]
}
```

#### GET /user/me

Get current user information including rating.

**Headers:**
```
Authorization: Bearer <auth_token>
```

**Response (200):**
```json
{
  "id": "user-uuid",
  "username": "student",
  "email": "",
  "rating": 1200,
  "gitea_repo_url": "http://gitea:3000/student/repo.git"
}
```

### Leaderboard

#### GET /grade/leaderboard/{hackathon}

Get top scorers for a given hackathon (no authentication required).

**Response (200):**
```json
{
  "entries": [
    {
      "username": "hermarti",
      "score": 70,
      "benchmark_ms": 286,
      "job_id": "job-uuid-...",
      "rating": 1200
    },
    {
      "username": "another-user",
      "score": 45,
      "benchmark_ms": 512,
      "job_id": "job-uuid-...",
      "rating": 1100
    }
  ]
}
```

#### GET /grade/suites

List available test suites (no authentication required).

**Response (200):**
```json
{
  "suites": [
    {
      "name": "libft",
      "active": true,
      "starts_at": "2024-01-01T00:00:00Z",
      "ends_at": "2024-12-31T23:59:59Z",
      "message": ""
    }
  ]
}
```

### Health

#### GET /health

Health check endpoint.

**Response (200):**
```json
{
  "status": "ok"
}
```

## Status Codes

| Code | Meaning | Description |
|------|---------|-------------|
| 200  | OK | Request successful |
| 201  | Created | Resource created (e.g., new user) |
| 202  | Accepted | Request accepted for processing |
| 400  | Bad Request | Invalid request format |
| 401  | Unauthorized | Authentication required or failed |
| 404  | Not Found | Resource not found |
| 500  | Internal Error | Server error |

## Job Status States

| Status | Meaning | Description |
|--------|---------|-------------|
| queued | Queued | Waiting for grader availability |
| processing | Processing | Running benchmarks and tests |
| completed | Completed | Grading finished successfully |
| failed | Failed | Grading encountered an error |
| error | Error | Server error during processing |

## Rate Limiting

Rate limiting is enforced per client using a token-bucket algorithm.

- **Limit:** 100 requests per minute per user (keyed by `Bearer` token when present, otherwise by IP address)
- **Headers:**
  - `X-RateLimit-Limit` — maximum requests per window
  - `X-RateLimit-Remaining` — requests remaining in current window
  - `X-RateLimit-Reset` — seconds until the window resets
- **429 Too Many Requests:** returned with `Retry-After` header when limit is exceeded

## Error Response Format

All error responses follow this format:

```json
{
  "error": "Description of the error",
  "code": "ERROR_CODE",
  "timestamp": "2024-06-03T10:30:00Z"
}
```

## Examples

### Complete Login and Submit Flow

```bash
# 1. Login
curl -X POST http://localhost:8000/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"student","password":"password"}'

# Response:
# {"token":"a1b2c3d4...","user":"student"}

# 2. Submit project
curl -X POST http://localhost:8000/api/v1/grade/submit \
  -H "Authorization: Bearer a1b2c3d4..." \
  -H "Content-Type: application/json" \
  -d '{"commit_sha":"abc123def456...","suite":"libft"}'

# Response:
# {"job_id":"job-uuid-1234-5678","status":"queued"}

# 3. Poll status
curl -X GET http://localhost:8000/api/v1/grade/status/job-uuid-1234-5678 \
  -H "Authorization: Bearer a1b2c3d4..."

# Response:
# {"job_id":"job-uuid-1234-5678","status":"processing","message":"Running tests..."}
```

## WebSocket Endpoints

### Connection

WebSocket connections use the same base host as the REST API. Authentication is via query parameter `token`.

```
ws://localhost:8000/ws/grade/status/{job_id}?token=<auth_token>
ws://localhost:8000/ws/grade/jobs?token=<auth_token>
```

### WS /ws/grade/status/{job_id}

Real-time job status updates. The server pushes `StatusResponse` JSON messages as the job progresses.

**Message format:**
```json
{
  "job_id": "job-uuid-1234",
  "status": "processing",
  "message": "Running tests...",
  "result": null
}
```

On completion, the server sends the final result and closes the connection:

```json
{
  "job_id": "job-uuid-1234",
  "status": "completed",
  "message": "Grading completed",
  "result": {
    "parser_success": true,
    "benchmark_ms": 150,
    "final_score": 95,
    "details": "All tests passed",
    "challenges": [...]
  }
}
```

### WS /ws/grade/jobs

Real-time jobs list for the authenticated user. The server pushes updated job lists every 5 seconds.

**Message format:**
```json
{
  "jobs": [
    {
      "job_id": "job-uuid-1234",
      "status": "completed",
      "message": "Grading completed",
      "created_at": "2024-06-03T10:30:00Z"
    }
  ]
}
```

### Implementation Notes

#### Client Fallback

The CLI automatically attempts WebSocket first. If the connection fails (e.g., server doesn't support WebSocket), it falls back to HTTP polling:

```go
wsc := client.NewWSClient(baseURL, token)
err := wsc.ListenStatus(jobID, func(s *StatusResponse) {
    // Real-time updates here
})
if err != nil {
    // Fallback to HTTP GET polling
}
```

#### Server Implementation

```go
http.HandleFunc("/ws/", handler.WSEndpoint(apiHandler))
```

The WebSocket handler supports both token-in-URL and Authorization header for flexibility.

## Implementation Notes

- All timestamps are in ISO 8601 format (UTC)
- Commit SHA must be valid git SHA-1 hash (40 characters, hex)
- Job ID is UUID v4 format
- Tokens are random 64-character hex strings (not JWT)
