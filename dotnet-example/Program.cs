using System;
using System.Net.Http;
using System.Threading.Tasks;
using CloudFauxntExample;

var cloudfauxntUrl = "http://localhost:8080";
var keyPairId = Environment.GetEnvironmentVariable("CLOUDFAUXNT_KEY_PAIR_ID") ?? "APKAJEXAMPLE123456";

// Find the private key - from dotnet-example directory, go up and then into keys
// dotnet-example/bin/Debug/net10.0/ -> ../../../ gets us to dotnet-example/ 
// then ../keys/private.pem gets us to the key
var baseDir = Path.GetDirectoryName(System.Reflection.Assembly.GetExecutingAssembly().Location) ?? ".";
var privateKeyPath = Path.GetFullPath(Path.Combine(baseDir, "..", "..", "..", "..", "keys", "private.pem"));

Console.WriteLine("╔════════════════════════════════════════════════════════╗");
Console.WriteLine("║          CloudFauxnt .NET Example Client               ║");
Console.WriteLine("╚════════════════════════════════════════════════════════╝\n");

// Example 1: Health check
Console.WriteLine("📋 Example 1: Health Check (Unsigned Request)");
Console.WriteLine("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━");
await TestHealthCheck(cloudfauxntUrl);
Console.WriteLine();

// Example 2: Unsigned request for file
Console.WriteLine("📋 Example 2: Unsigned File Request");
Console.WriteLine("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━");
await TestUnsignedRequest(cloudfauxntUrl);
Console.WriteLine();

// Check if we can do signing examples
if (File.Exists(privateKeyPath))
{
    var signer = new CloudFrontSigner(privateKeyPath, keyPairId);

    // Example 3: Signed URL
    Console.WriteLine("📋 Example 3: Signed URL Request");
    Console.WriteLine("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━");
    await TestSignedUrlRequest(cloudfauxntUrl, signer);
    Console.WriteLine();

    // Example 4: Signed Cookies
    Console.WriteLine("📋 Example 4: Signed Cookies Request");
    Console.WriteLine("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━");
    await TestSignedCookiesRequest(cloudfauxntUrl, signer);
    Console.WriteLine();
}
else
{
    Console.WriteLine("⚠️  Signing examples skipped (private key not found)");
    Console.WriteLine($"   Expected: {privateKeyPath}");
    Console.WriteLine("   Run: cd ../keys && openssl genrsa -out private.pem 2048 && openssl rsa -in private.pem -pubout -out public.pem");
    Console.WriteLine();
}

Console.WriteLine("✅ Examples complete!");

// =================================================================
// Helper Functions
// =================================================================

async Task TestHealthCheck(string baseUrl)
{
    using var client = new HttpClient();
    try
    {
        var response = await client.GetAsync($"{baseUrl}/health");
        var content = await response.Content.ReadAsStringAsync();
        Console.WriteLine($"✓ Status: {response.StatusCode}");
        Console.WriteLine($"✓ Response: {content}\n");
    }
    catch (Exception ex)
    {
        Console.WriteLine($"✗ Error: {ex.Message}\n");
    }
}

async Task TestUnsignedRequest(string baseUrl)
{
    using var client = new HttpClient();
    try
    {
        var url = $"{baseUrl}/s3/MyTestFile.txt";
        Console.WriteLine($"GET {url}");
        var response = await client.GetAsync(url);
        var content = await response.Content.ReadAsStringAsync();
        
        Console.WriteLine($"✓ Status: {response.StatusCode}");
        Console.WriteLine($"✓ Content-Type: {response.Content.Headers.ContentType}");
        Console.WriteLine($"✓ Content-Length: {content.Length} bytes");
        Console.WriteLine($"✓ Body: {content}");
        
        // Show some CloudFront headers
        if (response.Headers.TryGetValues("X-Amz-Cf-Id", out var cfId))
            Console.WriteLine($"✓ CloudFront ID: {string.Join(", ", cfId)}");
        Console.WriteLine();
    }
    catch (Exception ex)
    {
        Console.WriteLine($"✗ Error: {ex.Message}\n");
    }
}

async Task TestSignedUrlRequest(string baseUrl, CloudFrontSigner signer)
{
    using var client = new HttpClient();
    try
    {
        var resourcePath = "/s3/MyTestFile.txt";
        var expiresAt = DateTime.UtcNow.AddHours(1);
        
        var signedUrl = signer.GenerateSignedUrl(baseUrl, resourcePath, expiresAt);
        Console.WriteLine($"Generated signed URL (expires in 1 hour):\n{signedUrl}\n");
        
        var response = await client.GetAsync(signedUrl);
        var content = await response.Content.ReadAsStringAsync();
        
        Console.WriteLine($"✓ Status: {response.StatusCode}");
        Console.WriteLine($"✓ Content-Length: {content.Length} bytes");
        Console.WriteLine($"✓ Body: {content}");
        
        if (response.Headers.TryGetValues("X-Amz-Cf-Id", out var cfId))
            Console.WriteLine($"✓ CloudFront ID: {string.Join(", ", cfId)}");
        Console.WriteLine();
    }
    catch (Exception ex)
    {
        Console.WriteLine($"✗ Error: {ex.Message}\n");
    }
}

async Task TestSignedCookiesRequest(string baseUrl, CloudFrontSigner signer)
{
    var handler = new HttpClientHandler();
    var cookieContainer = new System.Net.CookieContainer();
    handler.CookieContainer = cookieContainer;

    using var client = new HttpClient(handler);
    try
    {
        // Generate signed cookies
        var resourcePath = "/s3/*";  // Wildcard allows all /s3/* paths
        var expiresAt = DateTime.UtcNow.AddHours(1);
        var cookies = signer.GenerateSignedCookies(resourcePath, expiresAt);

        Console.WriteLine($"Generated signed cookies for resource: {resourcePath}");
        Console.WriteLine($"Expires at: {expiresAt:yyyy-MM-dd HH:mm:ss} UTC\n");

        foreach (var cookie in cookies)
        {
            Console.WriteLine($"  {cookie.Key}: {cookie.Value.Substring(0, Math.Min(50, cookie.Value.Length))}...");
        }
        Console.WriteLine();

        // Add cookies to request
        var uri = new Uri($"{baseUrl}/s3/MyTestFile.txt");
        foreach (var cookie in cookies)
        {
            cookieContainer.Add(uri, new System.Net.Cookie(cookie.Key, cookie.Value));
        }

        // Make request with cookies
        Console.WriteLine($"GET {uri} (with signed cookies)\n");
        var response = await client.GetAsync(uri);
        var content = await response.Content.ReadAsStringAsync();

        Console.WriteLine($"✓ Status: {response.StatusCode}");
        Console.WriteLine($"✓ Content-Length: {content.Length} bytes");
        Console.WriteLine($"✓ Body: {content}");

        if (response.Headers.TryGetValues("X-Amz-Cf-Id", out var cfId))
            Console.WriteLine($"✓ CloudFront ID: {string.Join(", ", cfId)}");
        Console.WriteLine();
    }
    catch (Exception ex)
    {
        Console.WriteLine($"✗ Error: {ex.Message}\n");
    }
}
