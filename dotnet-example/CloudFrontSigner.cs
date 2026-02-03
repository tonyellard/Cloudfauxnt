using System;
using System.Collections.Generic;
using System.Linq;
using System.Security.Cryptography;
using System.Text;

namespace CloudFauxntExample;

/// <summary>
/// Generates CloudFront signed URLs and cookies using RSA-SHA1 signing.
/// Follows AWS CloudFront canned policy format.
/// </summary>
public class CloudFrontSigner
{
    private readonly RSA _privateKey;
    private readonly string _keyPairId;

    public CloudFrontSigner(string privateKeyPath, string keyPairId)
    {
        _privateKey = LoadPrivateKey(privateKeyPath);
        _keyPairId = keyPairId;
    }

    /// <summary>
    /// Generate a signed URL valid until the specified UTC expiration time.
    /// </summary>
    public string GenerateSignedUrl(string baseUrl, string resourcePath, DateTime expirationUtc)
    {
        var resourceUrl = $"{baseUrl}{resourcePath}";
        var policy = CreateCanedPolicy(resourceUrl, expirationUtc);
        var signature = SignPolicy(policy);

        // Build signed URL with query parameters
        var signedUrl = $"{resourceUrl}" +
            $"?Expires={ConvertToUnixTime(expirationUtc)}" +
            $"&Signature={signature}" +
            $"&Key-Pair-Id={_keyPairId}";

        return signedUrl;
    }

    /// <summary>
    /// Generate CloudFront signed cookies. Returns a dictionary of cookie name-value pairs.
    /// </summary>
    public Dictionary<string, string> GenerateSignedCookies(string resourcePath, DateTime expirationUtc)
    {
        // For cookies, the resource path can use wildcards like /*
        var policy = CreateCanedPolicy(resourcePath, expirationUtc);
        var signature = SignPolicy(policy);
        var expiresUnix = ConvertToUnixTime(expirationUtc);

        return new Dictionary<string, string>
        {
            { "CloudFront-Policy", policy },
            { "CloudFront-Signature", signature },
            { "CloudFront-Key-Pair-Id", _keyPairId }
        };
    }

    /// <summary>
    /// Create a canned policy JSON for CloudFront.
    /// </summary>
    private string CreateCanedPolicy(string resource, DateTime expirationUtc)
    {
        var expiresUnix = ConvertToUnixTime(expirationUtc);

        // Build policy as JSON string manually to handle special keys like AWS:EpochTime
        var policyJson = $@"{{
  ""Statement"": [
    {{
      ""Resource"": ""{EscapeJson(resource)}"",
      ""Condition"": {{
        ""DateLessThan"": {{
          ""AWS:EpochTime"": {expiresUnix}
        }}
      }}
    }}
  ]
}}";

        return Base64UrlEncode(policyJson);
    }

    /// <summary>
    /// Escape special characters in JSON strings.
    /// </summary>
    private static string EscapeJson(string str)
    {
        return str
            .Replace("\\", "\\\\")
            .Replace("\"", "\\\"")
            .Replace("\n", "\\n")
            .Replace("\r", "\\r");
    }

    /// <summary>
    /// Sign the policy using RSA-SHA1 with the private key.
    /// </summary>
    private string SignPolicy(string policy)
    {
        var policyBytes = Encoding.UTF8.GetBytes(policy);
        var signatureBytes = _privateKey.SignData(policyBytes, HashAlgorithmName.SHA1, RSASignaturePadding.Pkcs1);
        return Base64UrlEncode(signatureBytes);
    }

    /// <summary>
    /// Load RSA private key from PEM file.
    /// </summary>
    private static RSA LoadPrivateKey(string path)
    {
        var pem = File.ReadAllText(path);
        
        // Remove PEM headers and whitespace
        var lines = pem.Split(new[] { "\r\n", "\r", "\n" }, StringSplitOptions.None)
            .Where(l => !l.StartsWith("-----") && !string.IsNullOrWhiteSpace(l))
            .ToList();

        var base64 = string.Concat(lines);
        var keyBytes = Convert.FromBase64String(base64);

        var rsa = RSA.Create();
        
        try
        {
            // Try traditional PKCS#1 format (RSA PRIVATE KEY)
            rsa.ImportRSAPrivateKey(keyBytes, out _);
        }
        catch
        {
            // Try PKCS#8 format (PRIVATE KEY)
            try
            {
                rsa.ImportPkcs8PrivateKey(keyBytes, out _);
            }
            catch (Exception ex)
            {
                throw new InvalidOperationException($"Failed to load private key from {path}", ex);
            }
        }
        
        return rsa;
    }

    /// <summary>
    /// Convert DateTime to Unix timestamp (seconds since epoch).
    /// </summary>
    private static long ConvertToUnixTime(DateTime dateTime)
    {
        return (long)dateTime.Subtract(new DateTime(1970, 1, 1)).TotalSeconds;
    }

    /// <summary>
    /// Base64 URL encode (RFC 4648) - removes padding and uses URL-safe characters.
    /// </summary>
    private static string Base64UrlEncode(string text)
    {
        return Base64UrlEncode(Encoding.UTF8.GetBytes(text));
    }

    private static string Base64UrlEncode(byte[] bytes)
    {
        return Convert.ToBase64String(bytes)
            .Replace("+", "-")
            .Replace("/", "_")
            .TrimEnd('=');
    }
}
