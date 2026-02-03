# CloudFauxnt

A lightweight CloudFront emulator for local development, providing CloudFront-like features in front of S3 emulators or other backend services.

## Features

- ✅ **CloudFront Signed URLs** - Validate canned policy signed URLs with RSA-SHA1
- ✅ **CloudFront Signed Cookies** - Support for CloudFront-Policy, CloudFront-Signature, CloudFront-Key-Pair-Id
- ✅ **CORS Handling** - Full preflight and origin validation support
- ✅ **Multi-Origin Routing** - Route requests to different backends based on path patterns
- ✅ **CloudFront Headers** - Inject realistic CloudFront headers (X-Amz-Cf-Id, Via, X-Cache)
- 🐳 **Docker Ready** - Multi-stage Alpine builds, ~15MB final image
- 🔧 **Simple Configuration** - YAML-based static configuration

## Quick Start

### 1. Clone and Setup

```bash
cd Cloudfauxnt
cp config.example.yaml config.yaml
```

### 2. Start Services

CloudFauxnt and ess-three run as separate Docker containers on a shared network.

**Terminal 1: Start ess-three**
```bash
cd /path/to/ess-three
docker compose up -d
```

**Terminal 2: Start CloudFauxnt**
```bash
cd /path/to/CloudFauxnt
docker compose up -d

# Check health
curl http://localhost:8080/health
```

### 3. Generate RSA Keys (if using signing)

```bash
cd keys
openssl genrsa -out private.pem 2048
openssl rsa -in private.pem -pubout -out public.pem
cd ..
```

### 4. Configure

Edit `config.yaml` to match your environment:

```yaml
server:
  port: 8080
  host: "0.0.0.0"

origins:
  - name: s3
    url: http://ess-three:9000  # Service name for Docker
    path_patterns:
      - "/s3/*"
    strip_prefix: "/s3"
    target_prefix: "/test-bucket"

signing:
  enabled: true
  key_pair_id: "APKAJEXAMPLE123456"
  public_key_path: "/app/keys/public.pem"
```

## Examples

### .NET Example Client

A complete .NET 10 application demonstrating CloudFauxnt usage with unsigned requests, signed URLs, and signed cookies:

```bash
cd dotnet-example
dotnet run
```

Outputs:
- ✓ Health check via unsigned request
- ✓ File retrieval with path rewriting
- ✓ Signed URL generation and validation  
- ✓ Signed cookie generation and usage

See [dotnet-example/README.md](dotnet-example/README.md) for detailed documentation and code samples.

### Manual Testing with curl

**Health check:**
```bash
curl http://localhost:8080/health
# {"status":"healthy","service":"cloudfauxnt"}
```

**Unsigned request:**
```bash
curl http://localhost:8080/s3/MyTestFile.txt
# Hello World
```

**Signed URL request:**
```bash
# Generate signature using keys and policy
curl "http://localhost:8080/s3/file.txt?Expires=1234567890&Signature=...&Key-Pair-Id=APKAJEXAMPLE123456"
```

## Usage

### Path Rewriting

CloudFauxnt supports path rewriting to map incoming paths to different backend paths:

```yaml
origins:
  - name: myfiles
    url: http://ess-three:9000
    path_patterns:
      - "/s3/*"
    strip_prefix: "/s3"      # Remove this from the request path
    target_prefix: "/test-bucket"  # Add this to the proxied path
```

**Example flow:**
```
Client request:     http://localhost:8080/s3/document.pdf
Strip /s3:         /document.pdf
Add /test-bucket:  /test-bucket/document.pdf
Proxies to:        http://ess-three:9000/test-bucket/document.pdf
```

### Without Signature Validation

If signing is disabled in config, CloudFauxnt acts as a simple reverse proxy:

```bash
# Direct request (proxied to origin)
curl http://localhost:8080/s3/myfile.txt
```

### With Signed URLs

Enable signing in `config.yaml`:

```yaml
signing:
  enabled: true
  key_pair_id: "APKAJEXAMPLE123456"
  public_key_path: "/app/keys/public.pem"
```

Generate a signed URL using your private key:

```python
# Python example
import time
import base64
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding

def create_signed_url(url, key_pair_id, private_key_path, expires_in=3600):
    expires = int(time.time()) + expires_in
    policy = f"{url}?Expires={expires}"
    
    with open(private_key_path, 'rb') as f:
        private_key = serialization.load_pem_private_key(f.read(), password=None)
    
    signature = private_key.sign(policy.encode(), padding.PKCS1v15(), hashes.SHA1())
    encoded_sig = base64.b64encode(signature).decode()
    
    return f"{url}?Expires={expires}&Signature={encoded_sig}&Key-Pair-Id={key_pair_id}"

# Usage
signed_url = create_signed_url(
    "http://localhost:8080/bucket/myfile.txt",
    "APKAJEXAMPLE123456",
    "keys/private.pem"
)
print(signed_url)
```

Request the signed URL:

```bash
curl "http://localhost:8080/bucket/myfile.txt?Expires=1234567890&Signature=...&Key-Pair-Id=APKAJEXAMPLE123456"
```

### With CORS

CloudFauxnt handles CORS automatically:

```bash
# Preflight request
curl -X OPTIONS \
  -H "Origin: http://localhost:3000" \
  -H "Access-Control-Request-Method: GET" \
  http://localhost:8080/bucket/myfile.txt

# Actual request with Origin header
curl -H "Origin: http://localhost:3000" \
  http://localhost:8080/bucket/myfile.txt
```

## Configuration Reference

### Server Settings

```yaml
server:
  port: 8080              # Port to listen on
  host: "0.0.0.0"         # Host to bind to
  timeout_seconds: 30     # Request timeout
```

### Origins with Path Rewriting

Define backend services to proxy to with optional path rewriting:

```yaml
origins:
  - name: s3              # Friendly name
    url: http://ess-three:9000
    path_patterns:
      - "/s3/*"           # Match paths starting with /s3/
    strip_prefix: "/s3"  # Optional: remove this from request path
    target_prefix: "/test-bucket"  # Optional: add this to proxied path
  
  - name: api
    url: https://api.example.com
    path_patterns:
      - "/api/*"
```

**Pattern Matching:**
- Exact match: `/health` matches only `/health`
- Prefix wildcard: `/s3/*` matches `/s3/bucket/key`
- Catch-all: `/*` matches everything
- Longest pattern wins (first match if equal length)

### CORS

```yaml
cors:
  enabled: true
  allowed_origins:
    - "*"                           # Allow all (dev only!)
    - "http://localhost:3000"       # Specific origin
    - "*.example.com"               # Wildcard domain
  allowed_methods:
    - "GET"
    - "HEAD"
    - "OPTIONS"
    - "PUT"
    - "POST"
    - "DELETE"
  allowed_headers:
    - "*"                           # Allow all headers
  max_age: 3600                     # Preflight cache (seconds)
```

### Signing

```yaml
signing:
  enabled: true
  key_pair_id: "APKAJEXAMPLE123456"
  public_key_path: "/app/keys/public.pem"
```

## Integration with ess-three

CloudFauxnt is designed to work with [ess-three](../essthree), a lightweight S3 emulator.

### Separate Docker Containers (Recommended)

Run both as separate Docker services on a shared network:

**ess-three docker-compose.yml:**
```yaml
version: '3.8'
services:
  ess-three:
    build: .
    container_name: ess-three
    ports:
      - "9000:9000"
    volumes:
      - ./data:/data
    networks:
      - shared-network

networks:
  shared-network:
    driver: bridge
    name: shared-network
```

**CloudFauxnt docker-compose.yml:**
```yaml
version: '3.8'
services:
  cloudfauxnt:
    build: .
    container_name: cloudfauxnt
    ports:
      - "8080:8080"
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./keys:/app/keys:ro
    networks:
      - shared-network

networks:
  shared-network:
    driver: bridge
    name: shared-network
```

**Start both services:**
```bash
# Terminal 1
cd /path/to/ess-three && docker compose up -d

# Terminal 2
cd /path/to/CloudFauxnt && docker compose up -d

# Verify both are running
docker ps | grep -E "cloudfauxnt|ess-three"
```

**CloudFauxnt config.yaml:**
```yaml
origins:
  - name: s3
    url: http://ess-three:9000  # Service name for Docker network
    path_patterns:
      - "/*"
```

## Architecture

CloudFauxnt runs as a separate Docker container that proxies requests to origin services (like ess-three).

```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│  Docker Bridge Network (shared-network)                         │
│                                                                 │
│  ┌──────────────────────┐         ┌──────────────────────┐     │
│  │  CloudFauxnt (8080)  │         │  ess-three (9000)    │     │
│  │                      │         │                      │     │
│  │ • Validate Signature │         │ • S3 Emulator        │     │
│  │ • Check CORS         │─────────│ • Stores objects     │     │
│  │ • Rewrite paths      │         │   in ./data dir      │     │
│  │ • Proxy requests     │         │                      │     │
│  └──────────┬───────────┘         └──────────────────────┘     │
│             │                                                   │
│             │ http://ess-three:9000                             │
│             │ (Docker service name)                             │
│             │                                                   │
└─────────────┼───────────────────────────────────────────────────┘
              │
              │ Port mapping
              │
              ▼
        localhost:8080  (host access)
        localhost:9000
```

**Request Flow:**
1. Client → CloudFauxnt: `/s3/bucket/key?Signature=...`
2. CloudFauxnt validates signature and CORS
3. CloudFauxnt rewrites path: `/s3/bucket/key` → `/bucket/key` (using strip_prefix/target_prefix)
4. CloudFauxnt proxies to: `http://ess-three:9000/bucket/key`
5. ess-three returns object from local storage

**Key Points:**
- Both containers run on the same Docker bridge network (`shared-network`)
- CloudFauxnt accesses ess-three via service name `ess-three`, not localhost
- Path rewriting allows flexible routing (e.g., `/s3/*` → `/test-bucket/*`)
- Each service has its own docker-compose.yml file in separate directories

## Testing

### Run Unit Tests

```bash
go test ./...
```

### Integration Testing

```bash
# Start both services
cd /home/tony/Documents/essthree && docker compose up -d
cd /home/tony/Documents/Cloudfauxnt && docker compose up -d

# Wait for startup
sleep 2

# Test health endpoint
curl http://localhost:8080/health

# Test unsigned request with path rewriting
curl -v http://localhost:8080/s3/MyTestFile.txt

# Test CORS preflight
curl -X OPTIONS \
  -H "Origin: http://localhost:3000" \
  -H "Access-Control-Request-Method: GET" \
  -v http://localhost:8080/s3/MyTestFile.txt

# View logs
docker logs cloudfauxnt -f
docker logs ess-three -f
```

## Development

### Project Structure

```
Cloudfauxnt/
├── main.go              # Entry point, server setup
├── config.go            # Configuration parsing & validation
├── signing.go           # CloudFront signature validation
├── cors.go              # CORS middleware
├── handlers.go          # HTTP handlers and proxying
├── config.example.yaml  # Configuration template
├── Dockerfile           # Multi-stage Docker build
├── docker-compose.yml   # Container orchestration
├── go.mod              # Go dependencies
├── keys/
│   └── README.md       # Key generation instructions
└── test/
    └── integration_test.py  # Integration tests
```

### Building

```bash
# Local build
go build -o cloudfauxnt .

# Docker build
docker build -t cloudfauxnt:latest .

# Multi-platform build
docker buildx build --platform linux/amd64,linux/arm64 -t cloudfauxnt:latest .
```

## Troubleshooting

### Docker Network Connection Issues

**Containers can't reach each other:**
- Verify both containers are on the same network: `docker network ls` and `docker network inspect shared-network`
- Ensure both docker-compose.yml files define the same network: `networks: { shared-network: { driver: bridge, name: shared-network } }`
- Use service names (e.g., `http://ess-three:9000` from within CloudFauxnt) not `localhost` or `127.0.0.1`
- Check both services are running: `docker ps` should show both cloudfauxnt and ess-three containers

**Error: "dial tcp [::1]:9000: connect: connection refused"**
- This means Go resolved "localhost" to IPv6, but the origin only listens on IPv4
- Fix: Use `http://ess-three:9000` (Docker service name) instead of `http://localhost:9000` in config.yaml

**Testing connectivity:**
```bash
# From CloudFauxnt container
docker exec cloudfauxnt curl -v http://ess-three:9000/health

# From ess-three container
docker exec ess-three curl -v http://cloudfauxnt:8080/health
```

### Path Rewriting Not Working

**Requests still going to wrong path:**
- Verify `strip_prefix` and `target_prefix` are set in config.yaml origins section
- After changing config, restart CloudFauxnt: `docker compose restart cloudfauxnt`
- If code changes were made, rebuild without cache: `docker compose build --no-cache && docker compose up -d`
- Check logs to verify path transformation: `docker logs cloudfauxnt | grep "path"`

**Example configuration:**
```yaml
origins:
  - name: s3
    url: http://ess-three:9000
    path_patterns: ["/s3/*"]
    strip_prefix: "/s3"
    target_prefix: "/test-bucket"
```
This transforms `/s3/file.txt` → `/test-bucket/file.txt`

### Signature Validation Fails

- Verify the key pair ID matches between your signing code and config
- Check that the public key is valid: `openssl rsa -in public.pem -pubin -text`
- Ensure expiration time is in the future (Unix timestamp)
- Verify signature is base64-encoded correctly

### CORS Issues

- Check `allowed_origins` includes the requesting origin
- For development, use `["*"]` to allow all origins
- Verify the browser is sending an `Origin` header

### Origin Connection Fails (Non-Docker)

- For local development: ensure origin runs on correct port (e.g., `:9000`)
- For Docker: use service hostname, not localhost (see "Docker Network Connection Issues" above)
- Check origin service logs for errors: `docker logs ess-three -f`

### Windows/WSL Build Issues

If you see errors like "installsuffix" during Docker build:
- Ensure `.gitattributes` is present and enforcing LF line endings
- Run `git config core.autocrlf false` in the repository
- Clone the repository fresh after changing the setting

## Roadmap

- [ ] Custom CloudFront policies (beyond canned policy)
- [ ] IP address restrictions in policies
- [ ] Response caching with TTL
- [ ] Metrics and Prometheus integration
- [ ] Request/response logging
- [ ] TLS/HTTPS support
- [ ] Admin API for runtime inspection

## License

MIT License - See LICENSE file

## Related Projects

- [ess-three](../essthree) - Lightweight S3 emulator for local development
- [LocalStack](https://github.com/localstack/localstack) - Full AWS cloud stack emulator
- [Minio](https://github.com/minio/minio) - High-performance S3-compatible object storage

## Contributing

Contributions welcome! Please:
1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Submit a pull request

## Support

For issues and questions:
- GitHub Issues: [Report a bug](https://github.com/yourusername/cloudfauxnt/issues)
- Documentation: See this README and `keys/README.md`
