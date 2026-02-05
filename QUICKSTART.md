# CloudFauxnt Quick Start Guide

Welcome to CloudFauxnt! This guide will get you up and running in minutes.

## What is CloudFauxnt?

CloudFauxnt is a CloudFront emulator that adds CloudFront-like features to your local development:
- ✅ Signed URL validation
- ✅ CORS handling
- ✅ Multi-origin routing
- ✅ CloudFront headers

## Quick Start (3 Steps)

### 1. Create Configuration

```bash
# Copy the example config
cp config.example.yaml config.yaml
```

The default config proxies to `http://ess-three:9000` with path rewriting: `/s3/*` → `/test-bucket/*`

### 2. Run Both Services in Docker

CloudFauxnt and ess-three run as separate Docker containers connected via a shared network.

**Terminal 1: Start ess-three**
```bash
cd /path/to/ess-three
docker compose up -d
```

**Terminal 2: Start CloudFauxnt**
```bash
cd /path/to/CloudFauxnt
docker compose up -d
```

**Check they're both running:**
```bash
docker ps
# Should show both cloudfauxnt and ess-three containers
```

### 3. Test It

```bash
# Health check
curl http://localhost:8080/health
# Should return: {"status":"healthy","service":"cloudfauxnt"}

# Test proxy with path rewriting
curl http://localhost:9001/s3/MyTestFile.txt
# This proxies to: http://ess-three:9000/test-bucket/MyTestFile.txt
```

## Configuration Basics

Edit `config.yaml` to customize CloudFauxnt's behavior:

```yaml
server:
  port: 9001  # CloudFauxnt listens on this port
  # Optional: serve this object when requesting "/"
  default_root_object: "index.html"
  
origins:
  - name: s3
    url: http://ess-three:9000  # ess-three service name (Docker)
    path_patterns:
      - "/s3/*"         # Match paths starting with /s3/
    strip_prefix: "/s3"      # Remove /s3 from the path
    target_prefix: "/test-bucket"  # Add /test-bucket to the path
      
cors:
  enabled: true
  allowed_origins: ["*"]  # Allow all origins (dev only!)
  
signing:
  enabled: false  # Set to true to enable CloudFront signing
```

**Path Rewriting Example:**
```
Request:  http://localhost:9001/s3/MyTestFile.txt
Stripped: /MyTestFile.txt
Result:   http://ess-three:9000/test-bucket/MyTestFile.txt
```

## Running Separate Containers

CloudFauxnt and ess-three are separate Docker services that communicate via a shared Docker network (`shared-network`):

```bash
# Start ess-three (from ess-three directory)
cd /path/to/ess-three && docker compose up -d

# Start CloudFauxnt (from Cloudfauxnt directory)
cd /path/to/CloudFauxnt && docker compose up -d

# View logs
docker logs cloudfauxnt -f
docker logs ess-three -f

# Stop services
docker compose down  # (from either directory)

# Restart
docker compose restart
```

**Hostname Resolution:**
- Inside Docker: `http://ess-three:9000` (service name)
- From host machine: `http://localhost:9000` (port mapping)
- CloudFauxnt config uses: `http://ess-three:9000` (runs in Docker)

## Running Locally (No Docker)

You can also run both services locally without Docker:

**Terminal 1: ess-three**
```bash
cd /path/to/ess-three
./ess-three
```

**Terminal 2: CloudFauxnt**
```bash
cd /path/to/CloudFauxnt
go build -o cloudfauxnt .
./cloudfauxnt --config config.yaml
```

**Update config.yaml:**
```yaml
origins:
  - name: s3
    url: http://127.0.0.1:9000  # Use IPv4 address
    path_patterns:
      - "/s3/*"
    strip_prefix: "/s3"
    target_prefix: "/test-bucket"
```

## Next Steps

### Enable CloudFront Signing

1. **Generate RSA keys:**
   ```bash
   cd /path/to/CloudFauxnt/keys
   openssl genrsa -out private.pem 2048
   openssl rsa -in private.pem -pubout -out public.pem
   cd ..
   ```

2. **Update config.yaml:**
   ```yaml
   signing:
     enabled: true
     key_pair_id: "APKAJEXAMPLE123456"
     public_key_path: "/app/keys/public.pem"  # Path inside Docker container
     
     # Optional: Configure token behavior
     token_options:
       clock_skew_seconds: 30          # Allow 30-second time tolerance
       default_url_ttl_seconds: 3600   # 1-hour default TTL
       default_cookie_ttl_seconds: 86400  # 24-hour default TTL
   ```

3. **Rebuild and restart CloudFauxnt:**
   ```bash
   docker compose down
   docker compose build --no-cache
   docker compose up -d
   ```

4. **Generate and test a signed URL:**
   See [keys/README.md](keys/README.md) for Python example

**Token Options Explained:**
- `clock_skew_seconds` - Time tolerance for distributed systems where clocks differ slightly
- `default_url_ttl_seconds` - Default expiration time for signed URLs
- `default_cookie_ttl_seconds` - Default expiration time for signed cookies

### Add Multiple Origins

```yaml
origins:
  - name: s3
    url: http://localhost:9000
    path_patterns:
      - "/s3/*"
      - "/buckets/*"
      
  - name: api
    url: https://api.example.com
    path_patterns:
      - "/api/*"
      
  - name: default
    url: http://localhost:3000
    path_patterns:
      - "/*"  # Catch-all
```

Requests to `/api/users` go to `api.example.com`, `/s3/bucket/key` goes to S3, everything else goes to localhost:3000.

### Mixed Security Levels (Per-Origin Signatures)

When `signing.enabled: true`, you can allow unsigned access to specific origins:

```yaml
signing:
  enabled: true
  # ... other settings ...

origins:
  - name: public-files
    url: http://ess-three:9000
    path_patterns: ["/public/*"]
    require_signature: false   # Allow unsigned downloads
  
  - name: premium-content
    url: http://ess-three:9000
    path_patterns: ["/premium/*"]
    require_signature: true    # Always require signed URLs/cookies
  
  - name: temp-downloads
    url: http://ess-three:9000
    path_patterns: ["/temp/*"]
    # Omit require_signature - inherits global signing.enabled
```

Now `/public/file.txt` works unsigned, but `/premium/file.txt` requires a signature.

### Configure CORS

```yaml
cors:
  enabled: true
  allowed_origins:
    - "http://localhost:3000"
    - "https://app.example.com"
  allowed_methods:
    - "GET"
    - "POST"
    - "PUT"
    - "DELETE"
  allowed_headers:
    - "Content-Type"
    - "Authorization"
  max_age: 3600
```

## Troubleshooting

### "Connection refused" when testing

- Check CloudFauxnt is running: `docker compose ps` or `ps aux | grep cloudfauxnt`
- Check port 9001 is not in use: `lsof -i :9001`

### "No origin found for path"

- Check your `path_patterns` in config.yaml
- Patterns match longest-first
- Use `/*` as a catch-all

### Signature validation fails

- Verify `key_pair_id` matches between config and signing code
- Check expiration time is in the future
- Ensure public key is valid: `openssl rsa -in keys/public.pem -pubin -text`
- If you see "signature expired" errors from distributed systems, increase `clock_skew_seconds` in `token_options`
- Check system time is synchronized: `date` should show correct time

### CORS errors in browser

- Check `allowed_origins` includes your app's origin
- Use `["*"]` for development
- Verify CORS is enabled in config

## Example: Full Stack with ess-three

```yaml
# docker-compose.yml
version: '3.8'

services:
  ess-three:
    image: essthree:latest
    ports:
      - "9000:9000"
    volumes:
      - ./data:/data
    networks:
      - app

  cloudfauxnt:
    build: .
    ports:
      - "9001:9001"
    volumes:
      - ./config.yaml:/app/config.yaml:ro
      - ./keys:/app/keys:ro
    depends_on:
      - ess-three
    networks:
      - app

networks:
  app:
```

Then your apps connect to `http://localhost:9001` instead of `http://localhost:9000`.

## More Resources

- [Full README](README.md) - Detailed documentation
- [keys/README.md](keys/README.md) - Signing key generation and usage
- [test/README.md](test/README.md) - Testing guide
- [config.example.yaml](config.example.yaml) - Annotated configuration

## Getting Help

- Check logs: `docker compose logs cloudfauxnt` or `journalctl -u cloudfauxnt`
- Test health: `curl http://localhost:9001/health`
- Verify config: `./cloudfauxnt --config config.yaml` (will show validation errors)

Happy coding! 🎉
