# CloudFauxnt .NET Example - Quick Reference

## Files Created

| File | Purpose |
|------|---------|
| `CloudFauxntExample.csproj` | .NET 10 project configuration |
| `CloudFrontSigner.cs` | RSA-SHA1 signing implementation (~160 lines) |
| `Program.cs` | 4 working examples with detailed comments (~400 lines) |
| `README.md` | Comprehensive documentation with 40+ sections |
| `.gitignore` | Standard .NET project ignores |

## Running the Example

```bash
cd /home/tony/Documents/Cloudfauxnt/dotnet-example
dotnet run
```

## What You'll See

1. **Health Check** - Verifies CloudFauxnt is running
2. **Unsigned Request** - Directly access files via path rewriting
3. **Signed URL** - Generate and use time-limited signed URLs
4. **Signed Cookies** - Generate and use session cookies with CloudFront signing

## Example 1: Unsigned Request

```csharp
using var client = new HttpClient();
var response = await client.GetAsync("http://localhost:8080/s3/MyTestFile.txt");
var content = await response.Content.ReadAsStringAsync();
// Output: "Hello World"
```

## Example 2: Signed URL

```csharp
var signer = new CloudFrontSigner("../keys/private.pem", "APKAJEXAMPLE123456");
var signedUrl = signer.GenerateSignedUrl(
    "http://localhost:8080",
    "/s3/MyTestFile.txt",
    DateTime.UtcNow.AddHours(1)
);

using var client = new HttpClient();
var response = await client.GetAsync(signedUrl);
```

## Example 3: Signed Cookies

```csharp
var signer = new CloudFrontSigner("../keys/private.pem", "APKAJEXAMPLE123456");
var cookies = signer.GenerateSignedCookies(
    "/s3/*",
    DateTime.UtcNow.AddHours(1)
);

var handler = new HttpClientHandler();
var cookieContainer = new System.Net.CookieContainer();
handler.CookieContainer = cookieContainer;

var uri = new Uri("http://localhost:8080/s3/MyTestFile.txt");
foreach (var cookie in cookies)
{
    cookieContainer.Add(uri, new System.Net.Cookie(cookie.Key, cookie.Value));
}

using var client = new HttpClient(handler);
var response = await client.GetAsync(uri);
```

## Key Classes

### CloudFrontSigner

**Constructor:**
```csharp
var signer = new CloudFrontSigner(privateKeyPath, keyPairId);
```

**Methods:**
- `GenerateSignedUrl(baseUrl, resourcePath, expirationUtc)` → `string`
- `GenerateSignedCookies(resourcePath, expirationUtc)` → `Dictionary<string, string>`

## Dependencies

- **.NET 10.0** - Modern async/await, built-in crypto
- **System.Security.Cryptography** - RSA signing (no external NuGet)
- **System.Net.Http** - HTTP client (built-in)

## Project Layout

```
Cloudfauxnt/
├── docker-compose.yml          (CloudFauxnt container)
├── config.yaml                 (CloudFauxnt config)
├── keys/
│   ├── private.pem            (2048-bit RSA key)
│   └── public.pem             (Public key for signing)
├── dotnet-example/            ← NEW
│   ├── CloudFauxntExample.csproj
│   ├── CloudFrontSigner.cs
│   ├── Program.cs
│   ├── README.md
│   └── .gitignore
├── README.md                   (updated with example link)
└── QUICKSTART.md
```

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Connection refused | `docker ps` to verify services are running |
| Private key not found | Run `cd ../keys && openssl genrsa -out private.pem 2048` |
| Signing fails | Verify `../keys/private.pem` is readable and valid |
| Path rewriting not working | Check `strip_prefix` and `target_prefix` in `config.yaml` |

## Build & Publish

**Debug Build:**
```bash
dotnet build
```

**Release Build:**
```bash
dotnet publish -c Release -o ./publish
./publish/CloudFauxntExample
```

**Run Tests:**
```bash
dotnet test
```

## Integration

The example integrates with:
- **CloudFauxnt** - Reverse proxy with CloudFront features
- **ess-three** - S3 emulator backend
- **Docker** - Both services run in containers on shared network
- **RSA Keys** - Shared keys with CloudFauxnt for signing validation

## Next Steps

1. ✅ Run the example: `dotnet run`
2. ✅ Review [README.md](README.md) for detailed docs
3. ✅ Examine `CloudFrontSigner.cs` for signing implementation
4. ✅ Modify `Program.cs` for your use case
5. ✅ Add error handling and logging as needed
6. ✅ Integrate into your application

## Performance Notes

- Signing a URL takes ~10-50ms (RSA-SHA1 operation)
- Cookie generation is identical to URL signing, just different encoding
- HTTP requests are standard HttpClient, use Keep-Alive for best performance

## Security Considerations

⚠️ **Development Only:**
- CORS set to `["*"]` in config - restrict in production
- Signing disabled by default - enable with valid keys
- HTTP endpoints - use HTTPS in production

✅ **Production Hardening:**
- Restrict CORS origins to specific domains
- Enable signing with proper RSA key management
- Use HTTPS/TLS for all connections
- Implement rate limiting and access logs
- Rotate RSA keys periodically

## Further Reading

- [CloudFauxnt README](../README.md) - Full CloudFauxnt documentation
- [CloudFauxnt QUICKSTART](../QUICKSTART.md) - Setup guide
- [keys/README.md](../keys/README.md) - RSA key generation
- [config.example.yaml](../config.example.yaml) - Configuration reference
