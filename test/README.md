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
