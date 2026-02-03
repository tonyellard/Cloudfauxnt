package main

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// SignatureValidator handles CloudFront signature validation
type SignatureValidator struct {
	publicKey *rsa.PublicKey
	keyPairID string
}

// NewSignatureValidator creates a new signature validator
func NewSignatureValidator(publicKey *rsa.PublicKey, keyPairID string) *SignatureValidator {
	return &SignatureValidator{
		publicKey: publicKey,
		keyPairID: keyPairID,
	}
}

// ValidateRequest checks if a request has a valid CloudFront signature
func (sv *SignatureValidator) ValidateRequest(r *http.Request) error {
	// Check for signed URL parameters
	if r.URL.Query().Has("Signature") {
		return sv.validateSignedURL(r)
	}

	// Check for signed cookies
	if _, err := r.Cookie("CloudFront-Signature"); err == nil {
		return sv.validateSignedCookies(r)
	}

	// No signature found
	return fmt.Errorf("no CloudFront signature found")
}

// validateSignedURL validates a canned policy signed URL
func (sv *SignatureValidator) validateSignedURL(r *http.Request) error {
	query := r.URL.Query()

	// Extract required parameters
	signature := query.Get("Signature")
	expires := query.Get("Expires")
	keyPairID := query.Get("Key-Pair-Id")

	if signature == "" || expires == "" || keyPairID == "" {
		return fmt.Errorf("missing required signature parameters")
	}

	// Verify key pair ID matches
	if keyPairID != sv.keyPairID {
		return fmt.Errorf("invalid key pair ID: %s", keyPairID)
	}

	// Parse expiration time
	expiresInt, err := strconv.ParseInt(expires, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid Expires parameter: %w", err)
	}

	// Check if expired
	if time.Now().Unix() > expiresInt {
		return fmt.Errorf("signed URL has expired")
	}

	// Build canonical resource string (URL without signature params)
	canonicalURL := sv.buildCanonicalURL(r)

	// Decode base64 signature
	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	// Build policy string for canned policy
	policyStr := fmt.Sprintf("%s?Expires=%s", canonicalURL, expires)

	// Verify signature
	if err := sv.verifySignature(policyStr, sigBytes); err != nil {
		return fmt.Errorf("signature verification failed: %w", err)
	}

	return nil
}

// validateSignedCookies validates CloudFront signed cookies
func (sv *SignatureValidator) validateSignedCookies(r *http.Request) error {
	// Extract cookies
	policyCookie, err := r.Cookie("CloudFront-Policy")
	if err != nil {
		return fmt.Errorf("missing CloudFront-Policy cookie")
	}

	signatureCookie, err := r.Cookie("CloudFront-Signature")
	if err != nil {
		return fmt.Errorf("missing CloudFront-Signature cookie")
	}

	keyPairIDCookie, err := r.Cookie("CloudFront-Key-Pair-Id")
	if err != nil {
		return fmt.Errorf("missing CloudFront-Key-Pair-Id cookie")
	}

	// Verify key pair ID
	if keyPairIDCookie.Value != sv.keyPairID {
		return fmt.Errorf("invalid key pair ID in cookie: %s", keyPairIDCookie.Value)
	}

	// Decode policy (URL-safe base64)
	policy := strings.ReplaceAll(policyCookie.Value, "-", "+")
	policy = strings.ReplaceAll(policy, "_", "/")
	policy = strings.ReplaceAll(policy, "~", "=")

	policyBytes, err := base64.StdEncoding.DecodeString(policy)
	if err != nil {
		return fmt.Errorf("failed to decode policy: %w", err)
	}

	// Decode signature (URL-safe base64)
	signature := strings.ReplaceAll(signatureCookie.Value, "-", "+")
	signature = strings.ReplaceAll(signature, "_", "/")
	signature = strings.ReplaceAll(signature, "~", "=")

	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	// Verify signature against policy
	if err := sv.verifySignature(string(policyBytes), sigBytes); err != nil {
		return fmt.Errorf("cookie signature verification failed: %w", err)
	}

	// TODO: Parse policy JSON and check expiration
	// For MVP, we'll accept valid signatures

	return nil
}

// buildCanonicalURL constructs the canonical resource URL
func (sv *SignatureValidator) buildCanonicalURL(r *http.Request) string {
	// Get base URL without query parameters
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	host := r.Host
	path := r.URL.Path

	return fmt.Sprintf("%s://%s%s", scheme, host, path)
}

// verifySignature verifies an RSA-SHA1 signature
func (sv *SignatureValidator) verifySignature(message string, signature []byte) error {
	// Compute SHA1 hash of message
	hashed := sha1.Sum([]byte(message))

	// Verify RSA signature
	err := rsa.VerifyPKCS1v15(sv.publicKey, crypto.SHA1, hashed[:], signature)
	if err != nil {
		return fmt.Errorf("RSA verification failed: %w", err)
	}

	return nil
}

// RemoveSignatureParams removes CloudFront signature parameters from URL
func RemoveSignatureParams(u *url.URL) *url.URL {
	query := u.Query()
	query.Del("Signature")
	query.Del("Expires")
	query.Del("Key-Pair-Id")
	query.Del("Policy")

	cleaned := *u
	cleaned.RawQuery = query.Encode()
	return &cleaned
}
