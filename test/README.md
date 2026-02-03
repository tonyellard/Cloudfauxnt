# CloudFauxnt Testing

This directory contains integration and end-to-end tests for CloudFauxnt.

## Integration Tests

### Prerequisites

```bash
# Install Python dependencies
pip install requests cryptography
```

### Running Tests

```bash
# Start CloudFauxnt
cd ..
docker compose up -d

# Run integration tests
python integration_test.py

# Run per-origin signature enforcement tests
python test_per_origin_signing.py
```

### Test Coverage

The integration tests verify:

1. **Health Check** - `/health` endpoint returns 200 with healthy status
2. **CloudFront Headers** - Responses include `X-Amz-Cf-Id`, `Via`, `Server` headers
3. **CORS Preflight** - OPTIONS requests return proper CORS headers
4. **CORS Actual Requests** - GET/POST requests include CORS headers when origin is present
5. **Unsigned Requests** - Requests without signatures (behavior depends on config)
6. **Valid Signed URLs** - Properly signed URLs are accepted
7. **Expired Signed URLs** - Expired signatures return 403 Forbidden
8. **Clock Skew Tolerance** - URLs expiring within clock_skew_seconds are accepted
9. **Signed Cookies** - CloudFront-Policy cookies with valid signatures are accepted
10. **Expired Cookies** - Cookies with expired policies return 403 Forbidden

### Per-Origin Signature Enforcement Tests

The `test_per_origin_signing.py` script tests mixed security levels:

```bash
# Test with this config
# signing:
#   enabled: true
# origins:
#   - name: public
#     path_patterns: ["/public/*"]
#     require_signature: false
#   - name: private
#     path_patterns: ["/private/*"]
#     require_signature: true

python test_per_origin_signing.py
```

Tests include:
- ✅ Public paths accept unsigned requests
- ✅ Private paths reject unsigned requests  
- ✅ Private paths accept valid signatures
- ✅ Private paths reject expired signatures

## Manual Testing

### Test Unsigned Request

```bash
# If signing is disabled in config
curl -v http://localhost:8080/test-bucket/test-file.txt

# If signing is enabled, expect 403
```

### Test CORS

```bash
# Preflight request
curl -X OPTIONS \
  -H "Origin: http://localhost:3000" \
  -H "Access-Control-Request-Method: GET" \
  -H "Access-Control-Request-Headers: Content-Type" \
  -v http://localhost:8080/test-bucket/test-file.txt

# Actual request with origin
curl -H "Origin: http://localhost:3000" \
  -v http://localhost:8080/test-bucket/test-file.txt
```

### Test Signed URL

First, generate a signed URL using the Python script:

```python
from test.integration_test import CloudFauxntTester

tester = CloudFauxntTester()
tester.load_private_key("keys/private.pem")
signed_url = tester.create_signed_url("/test-bucket/test-file.txt")
print(signed_url)
```

Then test it:

```bash
curl -v "http://localhost:8080/test-bucket/test-file.txt?Expires=...&Signature=...&Key-Pair-Id=..."
```

### Test Expiration and Clock Skew

CloudFauxnt validates token expiration with configurable clock skew tolerance:

```bash
# Test a URL that expires in 25 seconds
# (should pass with default 30-second clock skew)
EXPIRES=$(($(date +%s) + 25))
python -c "
from integration_test import CloudFauxntTester
t = CloudFauxntTester()
t.load_private_key('../keys/private.pem')
url = t.create_signed_url('/test-bucket/test-file.txt', expires=$EXPIRES)
print(url)
" | xargs curl -v

# Test an already-expired URL
# (should fail with 403)
EXPIRES=$(($(date +%s) - 60))
python -c "
from integration_test import CloudFauxntTester
t = CloudFauxntTester()
t.load_private_key('../keys/private.pem')
url = t.create_signed_url('/test-bucket/test-file.txt', expires=$EXPIRES)
print(url)
" | xargs curl -v
```

To adjust clock skew tolerance, edit `config.yaml`:

```yaml
signing:
  token_options:
    clock_skew_seconds: 60  # Allow 60-second tolerance instead of 30
```

Then restart CloudFauxnt: `docker compose restart cloudfauxnt`

## Testing with ess-three

To test the full stack with the S3 emulator:

```bash
# Start both services
cd ..
docker compose up -d

# Create a test bucket and upload file to ess-three
aws --endpoint-url=http://localhost:9000 s3 mb s3://test-bucket
echo "Hello CloudFauxnt!" > test-file.txt
aws --endpoint-url=http://localhost:9000 s3 cp test-file.txt s3://test-bucket/

# Test via CloudFauxnt (unsigned, if signing disabled)
curl http://localhost:8080/test-bucket/test-file.txt

# Test via CloudFauxnt (signed)
python -c "
from integration_test import CloudFauxntTester
t = CloudFauxntTester()
t.load_private_key('../keys/private.pem')
url = t.create_signed_url('/test-bucket/test-file.txt')
print(url)
" | xargs curl
```

## Troubleshooting

### Connection Refused

```bash
# Check if CloudFauxnt is running
docker compose ps

# Check logs
docker compose logs cloudfauxnt
```

### 403 Forbidden

- Check if signing is enabled in config
- Verify the key pair ID matches between config and signing code
- Check if the signed URL has expired

### CORS Errors

- Verify CORS is enabled in config
- Check that your origin is in the `allowed_origins` list
- Use `["*"]` for development to allow all origins

### Origin Not Found

- Check the `origins` configuration
- Verify path patterns match your request path
- Check logs to see which origin was selected
