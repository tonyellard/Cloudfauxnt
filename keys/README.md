# CloudFront Signing Keys

This directory contains the RSA key pair used for CloudFront signed URL and cookie validation.

## Generating Keys

To generate a new RSA key pair for CloudFront signing:

```bash
# Generate private key (2048-bit RSA)
openssl genrsa -out private.pem 2048

# Extract public key from private key
openssl rsa -in private.pem -pubout -out public.pem

# View the public key (optional)
openssl rsa -in public.pem -pubin -text -noout
```

## Key Files

- `private.pem` - RSA private key (used for signing URLs/cookies in your application)
- `public.pem` - RSA public key (used by CloudFauxnt to validate signatures)

## Security Notes

⚠️ **Important:**
- Keep `private.pem` secure and NEVER commit it to version control
- The `.gitignore` file is configured to exclude `*.pem` and `*.key` files
- For local development, the security is relaxed, but treat keys carefully
- In production scenarios, use proper key management services

## Using Keys with CloudFauxnt

1. Generate keys using the commands above
2. Place `public.pem` in this directory
3. Update `config.yaml` with the correct `public_key_path` and `key_pair_id`
4. Keep `private.pem` in your application code for generating signed URLs

## CloudFront Key Pair ID

The `key_pair_id` in the config should match the identifier used when generating signed URLs. For local development, you can use any identifier (e.g., "APKAJEXAMPLE123456"), but ensure it matches between your signing code and CloudFauxnt configuration.

## Example: Generating a Signed URL (Python)

```python
import time
import base64
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding
from cryptography.hazmat.backends import default_backend

def create_signed_url(url, key_pair_id, private_key_path, expires_in=3600):
    # Calculate expiration time
    expires = int(time.time()) + expires_in
    
    # Create policy string
    policy = f"{url}?Expires={expires}"
    
    # Load private key
    with open(private_key_path, 'rb') as key_file:
        private_key = serialization.load_pem_private_key(
            key_file.read(),
            password=None,
            backend=default_backend()
        )
    
    # Sign the policy
    signature = private_key.sign(
        policy.encode('utf-8'),
        padding.PKCS1v15(),
        hashes.SHA1()
    )
    
    # Base64 encode signature
    encoded_signature = base64.b64encode(signature).decode('utf-8')
    
    # Build signed URL
    signed_url = f"{url}?Expires={expires}&Signature={encoded_signature}&Key-Pair-Id={key_pair_id}"
    
    return signed_url
```

## Verification

You can verify your key pair is valid:

```bash
# Create a test message
echo "test message" > test.txt

# Sign with private key
openssl dgst -sha1 -sign private.pem -out signature.bin test.txt

# Verify with public key
openssl dgst -sha1 -verify public.pem -signature signature.bin test.txt
# Should output: Verified OK
```
