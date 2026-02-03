#!/usr/bin/env python3
"""
Integration tests for CloudFauxnt
Tests signature validation, CORS, and proxying functionality
"""

import time
import base64
import requests
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding, rsa
from cryptography.hazmat.backends import default_backend


class CloudFauxntTester:
    def __init__(self, base_url="http://localhost:8080", key_pair_id="APKAJEXAMPLE123456"):
        self.base_url = base_url
        self.key_pair_id = key_pair_id
        self.private_key = None
        
    def load_private_key(self, path="../keys/private.pem"):
        """Load RSA private key from file"""
        with open(path, 'rb') as f:
            self.private_key = serialization.load_pem_private_key(
                f.read(),
                password=None,
                backend=default_backend()
            )
    
    def create_signed_url(self, path, expires_in=3600):
        """Generate a CloudFront canned policy signed URL"""
        if not self.private_key:
            raise ValueError("Private key not loaded")
        
        # Calculate expiration
        expires = int(time.time()) + expires_in
        
        # Build canonical URL
        url = f"{self.base_url}{path}"
        
        # Create policy string for canned policy
        policy = f"{url}?Expires={expires}"
        
        # Sign the policy
        signature = self.private_key.sign(
            policy.encode('utf-8'),
            padding.PKCS1v15(),
            hashes.SHA1()
        )
        
        # Base64 encode signature
        encoded_signature = base64.b64encode(signature).decode('utf-8')
        
        # Build signed URL
        signed_url = f"{url}?Expires={expires}&Signature={encoded_signature}&Key-Pair-Id={self.key_pair_id}"
        
        return signed_url
    
    def test_health_check(self):
        """Test the health endpoint"""
        print("Testing health check...")
        response = requests.get(f"{self.base_url}/health")
        assert response.status_code == 200, f"Expected 200, got {response.status_code}"
        data = response.json()
        assert data["status"] == "healthy", f"Expected healthy status, got {data}"
        print("✓ Health check passed")
    
    def test_unsigned_request(self):
        """Test request without signature (should fail if signing enabled)"""
        print("\nTesting unsigned request...")
        response = requests.get(f"{self.base_url}/test-bucket/test-key")
        # If signing is enabled, this should return 403
        # If signing is disabled, this will proxy to origin (may 404)
        print(f"  Status: {response.status_code}")
        print(f"  Response: {response.text[:200]}")
    
    def test_signed_url_valid(self):
        """Test valid signed URL"""
        print("\nTesting valid signed URL...")
        signed_url = self.create_signed_url("/test-bucket/test-key")
        print(f"  Signed URL: {signed_url[:100]}...")
        
        response = requests.get(signed_url)
        print(f"  Status: {response.status_code}")
        
        # If origin doesn't have the object, we'll get 404 from origin
        # But signature validation should pass (no 403)
        assert response.status_code != 403, "Signature validation failed (403)"
        print("✓ Signature validation passed")
    
    def test_signed_url_expired(self):
        """Test expired signed URL"""
        print("\nTesting expired signed URL...")
        
        # Create URL that expired 1 hour ago
        signed_url = self.create_signed_url("/test-bucket/test-key", expires_in=-3600)
        print(f"  Expired URL: {signed_url[:100]}...")
        
        response = requests.get(signed_url)
        print(f"  Status: {response.status_code}")
        
        # Should get 403 for expired signature
        assert response.status_code == 403, f"Expected 403 for expired URL, got {response.status_code}"
        print("✓ Expired signature correctly rejected")
    
    def test_cors_preflight(self):
        """Test CORS preflight request"""
        print("\nTesting CORS preflight...")
        
        headers = {
            'Origin': 'http://localhost:3000',
            'Access-Control-Request-Method': 'GET',
            'Access-Control-Request-Headers': 'Content-Type'
        }
        
        response = requests.options(f"{self.base_url}/test-bucket/test-key", headers=headers)
        print(f"  Status: {response.status_code}")
        print(f"  CORS Headers: {dict(response.headers)}")
        
        # Check for CORS headers
        assert 'Access-Control-Allow-Origin' in response.headers, "Missing CORS origin header"
        assert 'Access-Control-Allow-Methods' in response.headers, "Missing CORS methods header"
        print("✓ CORS preflight passed")
    
    def test_cors_actual_request(self):
        """Test actual request with CORS"""
        print("\nTesting CORS actual request...")
        
        headers = {
            'Origin': 'http://localhost:3000'
        }
        
        response = requests.get(f"{self.base_url}/test-bucket/test-key", headers=headers)
        print(f"  Status: {response.status_code}")
        
        # Check for CORS headers in response
        if 'Access-Control-Allow-Origin' in response.headers:
            print(f"  CORS Origin: {response.headers['Access-Control-Allow-Origin']}")
            print("✓ CORS headers present")
        else:
            print("  ⚠ No CORS headers (may be disabled in config)")
    
    def test_cloudfront_headers(self):
        """Test that CloudFront headers are added"""
        print("\nTesting CloudFront headers...")
        
        response = requests.get(f"{self.base_url}/health")
        
        # Check for CloudFront-like headers
        cf_headers = ['X-Amz-Cf-Id', 'Via', 'Server']
        for header in cf_headers:
            if header in response.headers:
                print(f"  {header}: {response.headers[header]}")
        
        assert 'Server' in response.headers, "Missing Server header"
        print("✓ CloudFront headers present")


def main():
    """Run all integration tests"""
    print("=" * 60)
    print("CloudFauxnt Integration Tests")
    print("=" * 60)
    
    tester = CloudFauxntTester()
    
    # Test 1: Health check (always works)
    try:
        tester.test_health_check()
    except Exception as e:
        print(f"✗ Health check failed: {e}")
        print("\nMake sure CloudFauxnt is running on http://localhost:8080")
        return
    
    # Test 2: CloudFront headers
    try:
        tester.test_cloudfront_headers()
    except Exception as e:
        print(f"✗ CloudFront headers test failed: {e}")
    
    # Test 3: CORS preflight
    try:
        tester.test_cors_preflight()
    except Exception as e:
        print(f"✗ CORS preflight failed: {e}")
    
    # Test 4: CORS actual request
    try:
        tester.test_cors_actual_request()
    except Exception as e:
        print(f"✗ CORS actual request failed: {e}")
    
    # Tests 5-7: Signature validation (requires keys)
    try:
        tester.load_private_key()
        
        tester.test_unsigned_request()
        tester.test_signed_url_valid()
        tester.test_signed_url_expired()
        
    except FileNotFoundError:
        print("\n⚠ Skipping signature tests - keys not found")
        print("  Generate keys with: cd keys && make keys")
    except Exception as e:
        print(f"\n✗ Signature tests failed: {e}")
    
    print("\n" + "=" * 60)
    print("Integration tests complete")
    print("=" * 60)


if __name__ == "__main__":
    main()
