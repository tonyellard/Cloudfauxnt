#!/usr/bin/env python3
"""
Test per-origin signature enforcement in CloudFauxnt.

This test demonstrates that different origins can have different
signature requirements, allowing mixed security levels.
"""

import requests
import time
import base64
import json
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding

BASE_URL = "http://localhost:9001"
KEY_PAIR_ID = "APKAJEXAMPLE123456"
PRIVATE_KEY_PATH = "../keys/private.pem"


def load_private_key():
    """Load the private key for signing."""
    try:
        with open(PRIVATE_KEY_PATH, "rb") as f:
            return serialization.load_pem_private_key(f.read(), password=None)
    except FileNotFoundError:
        print(f"⚠️  Private key not found at {PRIVATE_KEY_PATH}")
        print("    Run: cd ../keys && openssl genrsa -out private.pem 2048")
        return None


def create_signed_url(path, expires_in=3600):
    """Create a signed URL for a given path."""
    private_key = load_private_key()
    if not private_key:
        return None

    expires = int(time.time()) + expires_in
    policy = f"{BASE_URL}{path}?Expires={expires}"

    signature = private_key.sign(policy.encode(), padding.PKCS1v15(), hashes.SHA1())
    encoded_sig = base64.b64encode(signature).decode()

    return f"{path}?Expires={expires}&Signature={encoded_sig}&Key-Pair-Id={KEY_PAIR_ID}"


def test_public_path_unsigned():
    """Test that public paths work without signatures."""
    print("\n📋 Test 1: Public path (unsigned access)")
    print("━" * 50)
    
    # This should work because public/* has require_signature: false
    response = requests.get(f"{BASE_URL}/public/test-file.txt", allow_redirects=False)
    
    if response.status_code == 200:
        print("✅ Unsigned request to /public/* succeeded")
        return True
    elif response.status_code == 404:
        print("⚠️  Origin not found - public path not configured in this test")
        print("   Configure: require_signature: false for /public/* origin")
        return None
    else:
        print(f"❌ Unexpected status: {response.status_code}")
        print(f"   Response: {response.text[:100]}")
        return False


def test_private_path_unsigned():
    """Test that private paths reject unsigned requests."""
    print("\n📋 Test 2: Private path without signature")
    print("━" * 50)
    
    # This should fail with 403 because private/* has require_signature: true
    response = requests.get(f"{BASE_URL}/private/test-file.txt", allow_redirects=False)
    
    if response.status_code == 403:
        print("✅ Unsigned request to /private/* correctly rejected (403 Forbidden)")
        return True
    elif response.status_code == 404:
        print("⚠️  Origin not found - private path not configured in this test")
        print("   Configure: require_signature: true for /private/* origin")
        return None
    elif response.status_code == 200:
        print("❌ Unsigned request was allowed - should require signature")
        return False
    else:
        print(f"❌ Unexpected status: {response.status_code}")
        return False


def test_private_path_signed():
    """Test that private paths accept valid signatures."""
    print("\n📋 Test 3: Private path with valid signature")
    print("━" * 50)
    
    signed_url = create_signed_url("/private/test-file.txt")
    if not signed_url:
        print("⚠️  Cannot create signed URL - private key not available")
        return None
    
    response = requests.get(f"{BASE_URL}{signed_url}", allow_redirects=False)
    
    if response.status_code == 200:
        print("✅ Signed request to /private/* succeeded")
        return True
    elif response.status_code == 403:
        print("❌ Signed request was rejected - signature may be invalid")
        print(f"   Response: {response.text[:200]}")
        return False
    elif response.status_code == 404:
        print("⚠️  Origin not found or file doesn't exist")
        return None
    else:
        print(f"❌ Unexpected status: {response.status_code}")
        return False


def test_private_path_expired():
    """Test that private paths reject expired signatures."""
    print("\n📋 Test 4: Private path with expired signature")
    print("━" * 50)
    
    # Create a URL that expired 60 seconds ago
    signed_url = create_signed_url("/private/test-file.txt", expires_in=-60)
    if not signed_url:
        print("⚠️  Cannot create signed URL - private key not available")
        return None
    
    response = requests.get(f"{BASE_URL}{signed_url}", allow_redirects=False)
    
    if response.status_code == 403:
        print("✅ Expired signature correctly rejected (403 Forbidden)")
        return True
    elif response.status_code == 200:
        print("❌ Expired signature was accepted - should be rejected")
        return False
    elif response.status_code == 404:
        print("⚠️  Origin not found")
        return None
    else:
        print(f"❌ Unexpected status: {response.status_code}")
        return False


def main():
    """Run all tests."""
    print("\n" + "=" * 50)
    print("CloudFauxnt Per-Origin Signature Enforcement Tests")
    print("=" * 50)
    
    print("\n📝 Configuration Required:")
    print("   signing:")
    print("     enabled: true")
    print("   origins:")
    print("     - name: public")
    print("       url: http://ess-three:9000")
    print("       path_patterns: ['/public/*']")
    print("       require_signature: false")
    print("     - name: private")
    print("       url: http://ess-three:9000")
    print("       path_patterns: ['/private/*']")
    print("       require_signature: true")
    
    results = []
    
    # Run tests
    results.append(("Public path (unsigned)", test_public_path_unsigned()))
    results.append(("Private path (unsigned)", test_private_path_unsigned()))
    results.append(("Private path (signed)", test_private_path_signed()))
    results.append(("Private path (expired)", test_private_path_expired()))
    
    # Summary
    print("\n" + "=" * 50)
    print("Test Summary")
    print("=" * 50)
    
    passed = sum(1 for _, r in results if r is True)
    failed = sum(1 for _, r in results if r is False)
    skipped = sum(1 for _, r in results if r is None)
    
    for name, result in results:
        status = "✅ PASS" if result is True else "❌ FAIL" if result is False else "⚠️  SKIP"
        print(f"{status}: {name}")
    
    print(f"\nTotal: {passed} passed, {failed} failed, {skipped} skipped")
    
    if failed == 0:
        print("\n✅ All tests passed!" if passed > 0 else "\n⚠️  No tests ran")
    else:
        print(f"\n❌ {failed} test(s) failed")


if __name__ == "__main__":
    main()
