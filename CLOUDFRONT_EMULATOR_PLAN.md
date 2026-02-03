# CloudFront-lite Emulator: Architecture & Integration Plan

## Executive Summary

This document outlines a new **CloudFront-lite emulator** service to be added to the ess-three project. The emulator acts as a reverse proxy in front of the S3 emulator (and optionally other origins) to provide CloudFront-specific functionality for local development, including:

- **Signed URLs** (canned policy, custom policy)
- **Signed Cookies** (CloudFront-Policy, CloudFront-Signature, CloudFront-Key-Pair-Id)
- **CORS handling** (preflight, origin validation, headers)
- **Reverse proxy routing** (to multiple origins)

This is a **static configuration project** - developers write a YAML config file once and use it for extended periods, not a hot-reload scenario.

---

## Part 1: Ess-Three Project Context

### What is Ess-Three?

**Ess-Three** is a lightweight, S3-compatible object storage emulator written in **Go 1.23** for local development. It allows developers to test applications against S3 APIs without hitting AWS.

**Key characteristics:**
- Single-threaded HTTP server using **Chi router v5**
- Filesystem-based storage (JSON metadata + binary files)
- Runs in Docker (multi-stage Alpine builds, ~20MB final image)
- Listens on port 9000 (default S3 alternative port)
- Ignores authentication (intentional for dev/testing)

**Supported S3 operations:**
- Core: PutObject, GetObject, HeadObject, DeleteObject, ListObjects (V1 & V2)
- Advanced: Multipart uploads, Range requests (HTTP 206), Batch delete
- Metadata: Custom x-amz-meta-* headers, ETags, Content-Type preservation

### Ess-Three Architecture

```
essthree/
├── cmd/ess-three/main.go              # Entry point
├── internal/
│   ├── server/
│   │   ├── server.go                  # Chi router setup
│   │   └── handlers.go                # S3 API request handlers (540+ lines)
│   └── storage/
│       ├── storage.go                 # Storage interface & filesystem impl
│       └── storage_test.go
├── test/
│   ├── integration_test.py            # Python boto3 tests (6 tests)
│   └── advanced_features_test.py      # Advanced feature tests (6 tests)
├── dotnet-core-example/               # .NET 10 SDK example
├── Dockerfile                         # Multi-stage build
├── docker-compose.yml                 # Orchestration
├── go.mod / go.sum                    # Dependencies
└── README.md, GETTING_STARTED.md
```

### Important Ess-Three Quirks & Solutions

#### 1. **Windows/WSL Line Ending Issues**
- **Problem:** Git's `core.autocrlf` converts LF→CRLF, breaking Docker builds
- **Solution:** Added `.gitattributes` to force LF everywhere
- **For new agent:** If build fails with "installsuffix" error on Windows, this is the cause

#### 2. **Response Headers for SDK Compatibility**
- **Problem:** .NET SDK and some others expect specific S3 response headers
- **Solution:** Added `Server: ess-three`, `Date: [timestamp]`, `Connection: keep-alive`, `x-amz-version-id: null`
- **Location:** `internal/server/handlers.go` - applied to PutObject, GetObject, HeadObject, error responses

#### 3. **Chunked Transfer Encoding from .NET SDK**
- **Problem:** .NET SDK v3.7.300+ uses AWS Signature V4a with chunked transfer encoding on requests
- **Details:** Adds chunk signatures like `[hex-size];chunk-signature=[sig]\r\n[data]\r\n`
- **Not an issue:** Our emulator handles this correctly; it's SDK behavior
- **In tests:** Regex filters out chunk markers to extract actual data (see `dotnet-core-example/Program.cs`)

#### 4. **io.Copy vs io.CopyN**
- **Problem:** `io.Copy` can trigger unnecessary chunked encoding on responses with unknown size
- **Solution:** Use `io.CopyN(w, reader, metadata.Size)` to respect Content-Length
- **Location:** `internal/server/handlers.go` - GetObject handler (both normal and range requests)

#### 5. **Storage Path Handling**
- Objects stored as: `/data/{bucket}/objects/{key}`
- Metadata stored as: `/data/{bucket}/metadata/{key}.json`
- **Gotcha:** Deep nested keys create deeply nested directories; this is intentional and works fine

---

## Part 2: CloudFront-lite Emulator Design

### Overview

A **new Go service** that runs alongside ess-three as a reverse proxy, providing CloudFront-specific features. Acts as an intermediary between client requests and the S3 emulator (or other origins).

```
┌─────────────────────┐
│    Client App       │
│  (Python, .NET, etc)│
└──────────┬──────────┘
           │
           │ https://localhost:8080/myfile.txt?Signature=...&Expires=...
           │
    ┌──────▼─────────────────────┐
    │ CloudFront-lite Emulator    │
    │ (Go, Port 8080)             │
    │ ├─ Validate signatures      │
    │ ├─ Check CORS               │
    │ └─ Route to origin          │
    └──────┬─────────────────────┘
           │ (authenticated request)
           │ http://localhost:9000/bucket/key
           │
    ┌──────▼─────────────┐
    │ Ess-Three (S3 Emu) │
    │ (Go, Port 9000)    │
    │ (or other origin)   │
    └────────────────────┘
```

### Technology Stack

- **Language:** Go 1.23 (same as ess-three for consistency)
- **HTTP Router:** Chi v5 (same as ess-three)
- **Config:** YAML with file path mounting in Docker
- **Signing:** RSA-based CloudFront signature validation (crypto/rsa, crypto/sha1)
- **Time:** Canned policy expiration checking

### Configuration Approach

**Static YAML file** mounted as Docker volume. No hot-reload, no REST API. Designed for "set once, use for hours/days."

#### File Structure
```
essthree/
├── cloudfront-emulator/
│   ├── Dockerfile
│   ├── main.go                       # Entry point
│   ├── config.go                     # Config parsing
│   ├── handlers.go                   # Request handlers
│   ├── signing.go                    # Signature validation
│   ├── cors.go                       # CORS middleware
│   ├── config.yaml                   # ← Edit this (gitignored)
│   ├── config.example.yaml           # ← Template (in git)
│   ├── keys/                         # ← RSA signing keys (gitignored)
│   │   ├── private.pem
│   │   ├── public.pem
│   │   └── README.md                 # Key generation instructions
│   └── README.md
├── docker-compose.yml                # Updated to include CF emulator
└── .gitignore                        # Updated
```

#### Sample config.yaml

```yaml
server:
  port: 8080
  host: "0.0.0.0"
  timeout_seconds: 30

# Multiple origins with path-based routing
origins:
  - name: s3
    url: http://ess-three:9000
    path_patterns:
      - "/s3/*"
      - "/buckets/*"
    # Optional: required headers, auth headers, etc.
    
  - name: external-api
    url: https://api.example.com
    path_patterns:
      - "/api/*"

# CORS configuration
cors:
  enabled: true
  allowed_origins:
    - "*"  # Or: ["http://localhost:3000", "https://app.example.com"]
  allowed_methods: ["GET", "HEAD", "OPTIONS", "PUT", "POST", "DELETE"]
  allowed_headers: ["*"]
  max_age: 3600

# CloudFront signing
signing:
  enabled: true
  key_pair_id: "APKAJD7EXAMPLE"
  public_key_path: "/app/keys/public.pem"
  # If multiple keys needed in future, can expand to list
  
# Cache behavior (optional for MVP)
cache:
  enabled: false
  default_ttl_seconds: 300
```

#### Docker Integration

**Updated docker-compose.yml:**
```yaml
version: '3.8'
services:
  ess-three:
    build: .
    ports:
      - "9000:9000"
    volumes:
      - ./data:/data

  cloudfront-emulator:
    build: ./cloudfront-emulator
    ports:
      - "8080:8080"
    volumes:
      - ./cloudfront-emulator/config.yaml:/app/config.yaml:ro
      - ./cloudfront-emulator/keys:/app/keys:ro
    environment:
      - S3_ORIGIN=http://ess-three:9000
    depends_on:
      - ess-three
```

**Dockerfile:**
```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /build
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cloudfront-emulator

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /app
COPY --from=builder /build/cloudfront-emulator .
RUN mkdir -p /app/keys
EXPOSE 8080
CMD ["./cloudfront-emulator", "--config", "config.yaml"]
```

---

## Part 3: Implementation Details

### Core Features (MVP)

#### 1. Signed URLs (Canned Policy)
```go
// Validate CloudFront signed URL
GET /myfile.txt?Expires=1704067200&Signature=xxx&Key-Pair-Id=APKAJD7EXAMPLE

// Logic:
1. Extract Expires, Signature, Key-Pair-Id from query
2. Check if current time < Expires
3. Reconstruct canonical string: "GET\n/myfile.txt\nExpires=1704067200"
4. Verify RSA-SHA1 signature using public key
5. If valid: proxy to origin
6. If invalid: return 403 Forbidden with XML error
```

#### 2. Signed Cookies
```go
// Validate CloudFront signed cookies
Cookie: CloudFront-Policy=xxx; CloudFront-Signature=xxx; CloudFront-Key-Pair-Id=xxx

// Logic:
1. Extract cookies
2. Decode base64 policy
3. Validate expiration
4. Verify RSA-SHA1 signature
5. Check IP/user-agent restrictions (optional)
6. If valid: proxy request
```

#### 3. CORS Middleware
```go
// Handle OPTIONS preflight
OPTIONS /myfile.txt
Access-Control-Allow-Origin: http://localhost:3000
Access-Control-Allow-Methods: GET, HEAD, PUT, DELETE, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization
Access-Control-Max-Age: 3600

// Attach headers to actual requests
// Validate Origin against config
// Reject if not in allowed_origins list (unless "*")
```

#### 4. Request Routing
```go
// Path-based routing to origins
GET /s3/bucket/key → http://ess-three:9000/bucket/key
GET /api/users   → https://api.example.com/users

// Headers forwarded: Host, User-Agent, Accept*, etc.
// CloudFront headers added: X-Amz-Cf-Id, Via, X-Cache (optional)
```

### Package Structure

```go
// main.go
- Parse flags (--config)
- Load YAML config
- Initialize HTTP server
- Start listening

// config.go
- YAML unmarshaling
- Config validation
- Load public keys from filesystem

// handlers.go
- HTTP request handling
- Origin selection (path matching)
- Header forwarding
- Error responses (XML format)

// signing.go
- RSA-SHA1 signature verification
- Base64 decoding
- Time validation
- Policy parsing (JSON for custom policies)

// cors.go
- Middleware for CORS validation
- OPTIONS handler
- Response header injection
- Origin whitelist checking
```

### Key Implementation Decisions

#### Why RSA-SHA1?
CloudFront uses RSA-SHA1 for signed URLs/cookies. This is legacy but necessary for compatibility. Modern AWS Signature V4 is different (uses HMAC-SHA256 and is already handled by SDK on the client side).

#### Why YAML over JSON?
- More human-readable
- Easier to comment
- Git-friendly
- Standard in DevOps ecosystem

#### Why static config?
- Reduces complexity (no hot-reload logic)
- Forces intentional restarts (safer for testing)
- Matches developer workflow (set once, iterate on code)

#### Why separate Docker service?
- Keeps concerns separated
- Easy to disable (comment in docker-compose.yml)
- Can be shared with other S3-compatible backends
- Clear port separation (9000 = S3, 8080 = CloudFront)

---

## Part 4: Integration with Ess-Three

### No Changes Needed to Ess-Three Core
The CloudFront emulator is a **completely separate service**. Ess-Three continues to operate as-is on port 9000.

### Docker Compose Orchestration
Both services are defined in **updated docker-compose.yml**:
- Ess-Three: port 9000 (internal only, accessed via CloudFront)
- CloudFront: port 8080 (public-facing)
- Network: CloudFront can reach Ess-Three via `http://ess-three:9000`

### Testing Strategy

#### Unit Tests (Go)
```go
// Test signing validation
- Valid canned policy signature
- Expired policy
- Invalid signature
- Tampered key

// Test CORS
- Valid origin
- Invalid origin
- Missing Origin header
- OPTIONS preflight

// Test routing
- Path pattern matching
- Origin selection
- Header forwarding
```

#### Integration Tests (Python/Go)
```bash
# Start both services
docker compose up -d

# Test signed URL flow
python test/cloudfront_test.py

# Verify:
# 1. Valid signed URL → proxies to S3 → gets object
# 2. Invalid signed URL → 403 Forbidden
# 3. CORS preflight → correct headers
# 4. Multiple origins → correct routing
```

#### .NET Example (Optional)
Update `dotnet-core-example` to demonstrate CloudFront usage:
- Generate a signed URL
- Make request to CloudFront emulator (port 8080)
- Verify object is retrieved

---

## Part 5: Key Gotchas & Lessons Learned

### From Ess-Three Development

#### 1. **Windows Build Issues**
- **Solution Already In Place:** `.gitattributes` forces LF
- **For CloudFront:** Apply same line-ending strategy

#### 2. **SDK Compatibility is Fragile**
- Different SDK versions have different expectations
- Always test against multiple language SDKs
- Response headers matter (Server, Date, Connection, Version-ID)

#### 3. **Chunk Encoding Surprises**
- Some SDKs use chunked encoding even when Content-Length is known
- Use `io.CopyN` to respect declared sizes
- Don't assume raw HTTP responses = what SDK receives

#### 4. **Timezone Issues**
- CloudFront signatures use UTC timestamps
- Always work in UTC internally
- Format: `time.Now().UTC()` not `time.Now()`

### For CloudFront Emulator Specifically

#### 1. **Key Management is Security Theater for Local Dev**
- Use self-signed certificates if needed
- Private keys will be in gitignore but clearly stored
- Document key generation process
- Make it easy to swap keys for testing (don't hardcode)

#### 2. **Base64 Encoding Variations**
- CloudFront uses standard base64, not URL-safe variant
- Signatures use raw binary form before base64, not URL-encoded
- Be careful about `=` padding on decoded data

#### 3. **Policy Expiration Nuances**
- Canned policy: `Expires` parameter is Unix timestamp
- Custom policy: expiration in JSON is also Unix timestamp
- Always compare with `time.Now().Unix()` (not milliseconds)

#### 4. **Path Pattern Matching**
- Use glob patterns (e.g., `/s3/*`, `/api/v2/*`)
- Match longest pattern first (order matters in config)
- Test edge cases: `/`, `/s3`, `/s3/`, `/s3/bucket/key`

#### 5. **Origin Health Checks**
- CloudFront emulator doesn't check origin availability (MVP)
- If S3 emulator is down, proxy requests will timeout
- Document this - it's intentional for MVP
- Can add health checks in future iteration

---

## Part 6: Development Workflow

### Initial Setup
```bash
# 1. Start ess-three
docker compose up -d ess-three

# 2. Generate RSA keys (one-time)
cd cloudfront-emulator/keys
openssl genrsa -out private.pem 2048
openssl rsa -in private.pem -pubout -out public.pem
cd ../..

# 3. Create config
cp cloudfront-emulator/config.example.yaml cloudfront-emulator/config.yaml
# Edit config.yaml as needed

# 4. Start CloudFront emulator
docker compose up -d cloudfront-emulator
```

### Testing
```bash
# Run unit tests
cd cloudfront-emulator
go test ./...

# Run integration tests
python ../test/cloudfront_test.py

# Manual curl test
curl -v "http://localhost:8080/s3/test-bucket/test-key?Signature=xxx&Expires=xxx&Key-Pair-Id=xxx"
```

### Config Changes
```bash
# Edit config.yaml
nano cloudfront-emulator/config.yaml

# Restart service (no rebuild needed, file is mounted)
docker compose restart cloudfront-emulator

# Verify
curl http://localhost:8080/health  # Optional health endpoint
```

---

## Part 7: Handoff Checklist for New Agent

- [ ] Read ess-three README.md to understand existing project
- [ ] Understand that CloudFront emulator is a **new, separate service**
- [ ] Review Ess-Three quirks section (Windows builds, headers, chunking)
- [ ] Know that config is YAML, mounted via Docker, static (no hot-reload)
- [ ] Understand RSA-SHA1 is legacy but required for CloudFront compatibility
- [ ] Test against multiple SDKs (Python, .NET, Go) - they have different expectations
- [ ] Remember: CloudFront emulator should not modify ess-three code
- [ ] Use Go 1.23, Chi v5, same Docker patterns as ess-three for consistency
- [ ] Document key generation process clearly for users
- [ ] Add to main docker-compose.yml so both services start together

---

## Part 8: Future Enhancements (Out of Scope for MVP)

1. **Health checks** - `/health` endpoint for both services
2. **Request logging** - JSON structured logs to stdout
3. **Custom policy support** - Beyond canned policy
4. **IP restrictions** - Check originating IP against policy
5. **Cache behavior** - In-memory caching with TTLs
6. **Additional origins** - Support for Lambda, DynamoDB streams, etc.
7. **Admin API** - REST endpoint to view/validate config (no updates, would require restart)
8. **Metrics** - Prometheus metrics for requests, signatures, routing
9. **TLS support** - Self-signed cert support for HTTPS testing
10. **Request signing** - Option to sign outbound requests to origins (for origin auth)

---

## References & Related Code

**Ess-Three:**
- GitHub: https://github.com/{github-username}/Ess-Three
- Main handler: `internal/server/handlers.go` (540+ lines)
- Storage: `internal/storage/storage.go`
- Docker: `Dockerfile` (multi-stage build pattern to follow)

**CloudFront Documentation:**
- AWS CloudFront Signed URLs: https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/private-content-signed-urls.html
- Signed Cookies: https://docs.aws.amazon.com/AmazonCloudFront/latest/DeveloperGuide/private-content-signed-cookies.html

**Go Libraries Needed:**
- `crypto/rsa` - RSA key operations
- `crypto/sha1` - SHA1 hashing
- `encoding/base64` - Base64 encoding/decoding
- `gopkg.in/yaml.v3` - YAML parsing (chi already depends on similar)

**Testing:**
- Python: boto3 (already used in ess-three tests)
- Go: standard testing package
- Consider: testify/assert for cleaner test code

---

## Questions to Ask Before Starting Implementation

1. Should we validate that origins are reachable, or let failures propagate?
2. Do we need admin endpoints (view config, check status) beyond `--config` CLI?
3. Should we add request/response logging to stdout?
4. What CloudFront headers should we inject (X-Amz-Cf-Id, Via, X-Cache)?
5. Should we support origin-specific auth headers?
6. Do we need IP restriction validation in policies?

---

**Document Version:** 1.0  
**Date:** February 2, 2026  
**Status:** Architecture & Planning (Ready for Implementation)
