# CloudFauxnt .NET Example

A complete .NET 10 console application demonstrating how to use CloudFauxnt to fetch files with unsigned requests, signed URLs, and signed CloudFront cookies.

## Features

- ✅ **Unsigned Requests** - Direct file access without authentication
- ✅ **Signed URLs** - CloudFront-style signed URLs with expiration
- ✅ **Signed Cookies** - CloudFront-style signed cookies for session-based access
- ✅ **RSA-SHA1 Signing** - Implements CloudFront canned policy signing
- ✅ **Live Examples** - Working demonstrations against running CloudFauxnt service

## Prerequisites

- .NET 10.0 SDK or later
- CloudFauxnt running on `http://localhost:8080`
- ess-three S3 emulator running on `http://ess-three:9000` (Docker network) or `http://localhost:9000` (local)
- (Optional) RSA keys in `../keys/private.pem` and `../keys/public.pem` for signing examples

## Quick Start

### 1. Run CloudFauxnt and ess-three

```bash
# Terminal 1: Start ess-three
cd /home/tony/Documents/essthree
docker compose up -d

# Terminal 2: Start CloudFauxnt  
cd /home/tony/Documents/Cloudfauxnt
docker compose up -d
```

### 2. Generate RSA Keys (for signing examples)

```bash
cd /home/tony/Documents/Cloudfauxnt/keys
openssl genrsa -out private.pem 2048
openssl rsa -in private.pem -pubout -out public.pem
```

### 3. Run the Example

```bash
cd /home/tony/Documents/Cloudfauxnt/dotnet-example
dotnet run
```

## Example Output

```
╔════════════════════════════════════════════════════════╗
║          CloudFauxnt .NET Example Client               ║
╚════════════════════════════════════════════════════════╝

📋 Example 1: Health Check (Unsigned Request)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✓ Status: OK
✓ Response: {"status":"healthy","service":"cloudfauxnt"}

📋 Example 2: Unsigned File Request
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
GET http://localhost:8080/s3/MyTestFile.txt
✓ Status: OK
✓ Content-Type: text/plain
✓ Content-Length: 13 bytes
✓ Body: Hello World
✓ CloudFront ID: 22F7D60FA9464217BD451E3CCD650098

📋 Example 3: Signed URL Request
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Generated signed URL (expires in 1 hour):
http://localhost:8080/s3/MyTestFile.txt?Expires=1770137849&Signature=...&Key-Pair-Id=APKAJEXAMPLE123456

✓ Status: OK
✓ Content-Length: 13 bytes
✓ Body: Hello World

📋 Example 4: Signed Cookies Request
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Generated signed cookies for resource: /s3/*
Expires at: 2026-02-03 16:57:29 UTC

✓ Status: OK
✓ Content-Length: 13 bytes
✓ Body: Hello World

✅ Examples complete!
```

## Code Examples

### Example 1: Unsigned Request

Simply make an HTTP GET request to CloudFauxnt:

```csharp
using var client = new HttpClient();
var response = await client.GetAsync("http://localhost:8080/s3/MyTestFile.txt");
var content = await response.Content.ReadAsStringAsync();
Console.WriteLine(content);  // Output: "Hello World"
```

### Example 2: Signed URL

Generate a CloudFront-style signed URL that expires after 1 hour:

```csharp
var signer = new CloudFrontSigner("/path/to/private.pem", "APKAJEXAMPLE123456");

// Generate signed URL (expires in 1 hour)
var signedUrl = signer.GenerateSignedUrl(
    "http://localhost:8080",
    "/s3/MyTestFile.txt",
    DateTime.UtcNow.AddHours(1)
);

// Use the signed URL
using var client = new HttpClient();
var response = await client.GetAsync(signedUrl);
var content = await response.Content.ReadAsStringAsync();
```

The generated URL includes query parameters:
```
http://localhost:8080/s3/MyTestFile.txt?Expires=1770137849&Signature=...&Key-Pair-Id=APKAJEXAMPLE123456
```

### Example 3: Signed Cookies

Generate CloudFront-style signed cookies for session-based access:

```csharp
var signer = new CloudFrontSigner("/path/to/private.pem", "APKAJEXAMPLE123456");

// Generate cookies for /s3/* resources (expires in 1 hour)
var cookies = signer.GenerateSignedCookies(
    "/s3/*",
    DateTime.UtcNow.AddHours(1)
);

// Set up HTTP client with cookie container
var handler = new HttpClientHandler();
var cookieContainer = new System.Net.CookieContainer();
handler.CookieContainer = cookieContainer;
var client = new HttpClient(handler);

// Add cookies to the request
var uri = new Uri("http://localhost:8080/s3/MyTestFile.txt");
foreach (var cookie in cookies)
{
    cookieContainer.Add(uri, new System.Net.Cookie(cookie.Key, cookie.Value));
}

// Make request with cookies
var response = await client.GetAsync(uri);
var content = await response.Content.ReadAsStringAsync();
```

The cookies include:
- `CloudFront-Policy` - Base64-encoded JSON policy with resource and expiration
- `CloudFront-Signature` - RSA-SHA1 signature of the policy
- `CloudFront-Key-Pair-Id` - The key pair ID used to sign

## Project Structure

```
dotnet-example/
├── CloudFauxntExample.csproj  # Project file with .NET 10 target
├── CloudFrontSigner.cs        # Implements signed URL/cookie generation
├── Program.cs                 # Examples demonstrating all three access methods
└── bin/
    └── Debug/net10.0/        # Compiled output
```

## CloudFrontSigner Class

The `CloudFrontSigner` class handles all CloudFront signing operations:

### Constructor

```csharp
var signer = new CloudFrontSigner(privateKeyPath, keyPairId);
```

### Methods

#### GenerateSignedUrl(baseUrl, resourcePath, expirationUtc)

Generates a signed URL with query parameters.

```csharp
var signedUrl = signer.GenerateSignedUrl(
    "http://localhost:8080",
    "/s3/bucket/key.pdf",
    DateTime.UtcNow.AddHours(24)
);
```

#### GenerateSignedCookies(resourcePath, expirationUtc)

Generates a dictionary of signed cookies for session access.

```csharp
var cookies = signer.GenerateSignedCookies(
    "/s3/*",  // Wildcard matches all paths under /s3/
    DateTime.UtcNow.AddDays(7)
);

foreach (var cookie in cookies)
{
    Console.WriteLine($"{cookie.Key}: {cookie.Value}");
}
// Output:
// CloudFront-Policy: ewogICJTdGF0ZW1lbnQi...
// CloudFront-Signature: Ros2c3qj0yFY3I0Yu2lL...
// CloudFront-Key-Pair-Id: APKAJEXAMPLE123456
```

## How Path Rewriting Works

CloudFauxnt uses the configuration to rewrite paths before proxying to the backend:

```yaml
origins:
  - name: s3
    url: http://ess-three:9000
    path_patterns: ["/s3/*"]
    strip_prefix: "/s3"
    target_prefix: "/test-bucket"
```

When you request `/s3/MyTestFile.txt`:

1. CloudFauxnt matches `/s3/*` pattern
2. Strips `/s3` → `/MyTestFile.txt`
3. Adds `/test-bucket` → `/test-bucket/MyTestFile.txt`
4. Proxies to `http://ess-three:9000/test-bucket/MyTestFile.txt`
5. ess-three serves the file from its storage

## Building and Running

### Build

```bash
cd /home/tony/Documents/Cloudfauxnt/dotnet-example
dotnet build
```

### Run

```bash
dotnet run
```

### Publish (Release Build)

```bash
dotnet publish -c Release -o ./publish
./publish/CloudFauxntExample
```

## Environment Variables

- `CLOUDFAUXNT_KEY_PAIR_ID` - Override the default key pair ID (defaults to `APKAJEXAMPLE123456`)

```bash
CLOUDFAUXNT_KEY_PAIR_ID="APKAIJRANDOMSTRING123" dotnet run
```

## Troubleshooting

### "Connection refused" error

**Symptom:** Cannot connect to CloudFauxnt or ess-three

**Solutions:**
- Ensure both services are running: `docker ps | grep -E "cloudfauxnt|ess-three"`
- Check CloudFauxnt is listening: `curl http://localhost:8080/health`
- Check ess-three is listening: `curl http://localhost:9000/health`
- Verify network setup: `docker network inspect shared-network`

### "Private key not found" warning

**Symptom:** Signing examples are skipped with path warning

**Solutions:**
- Generate keys:
  ```bash
  cd /home/tony/Documents/Cloudfauxnt/keys
  openssl genrsa -out private.pem 2048
  openssl rsa -in private.pem -pubout -out public.pem
  ```
- Re-run the example: `dotnet run`

### "Failed to load private key" error

**Symptom:** Runtime exception when trying to use signing

**Solutions:**
- Verify the key file exists and is readable: `ls -la ../keys/private.pem`
- Verify the key format is valid:
  ```bash
  openssl rsa -in ../keys/private.pem -check
  ```
- Regenerate the key if corrupted:
  ```bash
  cd ../keys
  rm private.pem public.pem
  openssl genrsa -out private.pem 2048
  openssl rsa -in private.pem -pubout -out public.pem
  ```

### Signed URL returns 403 Forbidden

**Symptom:** Signed URL works when generated but CloudFauxnt rejects it

**Solutions:**
- Verify CloudFauxnt has signing enabled in `config.yaml`:
  ```yaml
  signing:
    enabled: true
    key_pair_id: "APKAJEXAMPLE123456"
    public_key_path: "/app/keys/public.pem"
  ```
- Verify the key pair ID matches between the signer and CloudFauxnt config
- Verify the expiration time is in the future (check system clock)
- Check CloudFauxnt logs: `docker logs cloudfauxnt -f`

## Integration with CloudFauxnt Config

The example uses the default CloudFauxnt configuration. To customize:

### 1. Update CloudFauxnt Config

Edit `/home/tony/Documents/Cloudfauxnt/config.yaml`:

```yaml
server:
  port: 8080

origins:
  - name: s3
    url: http://ess-three:9000
    path_patterns: ["/s3/*"]
    strip_prefix: "/s3"
    target_prefix: "/test-bucket"

signing:
  enabled: true
  key_pair_id: "APKAJEXAMPLE123456"
  public_key_path: "/app/keys/public.pem"
```

### 2. Update the Example

Modify `Program.cs` to use your configuration:

```csharp
var cloudfauxntUrl = "http://localhost:8080";
var keyPairId = "APKAJEXAMPLE123456";  // Match config.yaml
var resourcePath = "/s3/MyTestFile.txt";
```

### 3. Restart CloudFauxnt

```bash
cd /home/tony/Documents/Cloudfauxnt
docker compose restart cloudfauxnt
```

## Related Files

- [CloudFauxnt README](../README.md) - Main CloudFauxnt documentation
- [CloudFauxnt QUICKSTART](../QUICKSTART.md) - Setup guide
- [keys/README.md](../keys/README.md) - RSA key generation guide
- [config.example.yaml](../config.example.yaml) - Configuration template

## License

MIT License - Same as CloudFauxnt

## Contributing

Improvements to this example are welcome! Areas for enhancement:

- [ ] Async file downloads with progress tracking
- [ ] Batch operations (multiple files)
- [ ] Error handling and retry logic
- [ ] Integration tests
- [ ] Performance benchmarking
- [ ] Support for custom headers
- [ ] Range request support (partial downloads)
