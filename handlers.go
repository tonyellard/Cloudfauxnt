// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ProxyHandler handles incoming requests and proxies them to origins
type ProxyHandler struct {
	config    *Config
	validator *SignatureValidator
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler(config *Config, validator *SignatureValidator) *ProxyHandler {
	return &ProxyHandler{
		config:    config,
		validator: validator,
	}
}

// ServeHTTP handles the proxy request
func (ph *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Find matching origin first to determine signature requirement and default root object
	origin, err := ph.config.FindOrigin(r.URL.Path)
	if err != nil {
		ph.writeCloudFrontError(w, "NoSuchKey", "The specified path does not match any configured origin", http.StatusNotFound)
		return
	}

	// Determine if signature is required for this origin
	requireSignature := ph.config.Signing.Enabled // Default to global setting
	if origin.RequireSignature != nil {
		// Per-origin setting overrides global setting
		requireSignature = *origin.RequireSignature
	}

	// Validate signature if required
	if requireSignature {
		if err := ph.validator.ValidateRequest(r); err != nil {
			ph.writeCloudFrontError(w, "AccessDenied", err.Error(), http.StatusForbidden)
			return
		}
	}

	// Proxy to origin
	if err := ph.proxyToOrigin(w, r, origin); err != nil {
		ph.writeCloudFrontError(w, "ServiceUnavailable", err.Error(), http.StatusServiceUnavailable)
		return
	}
}

// proxyToOrigin forwards the request to the origin server
func (ph *ProxyHandler) proxyToOrigin(w http.ResponseWriter, r *http.Request, origin *Origin) error {
	// Parse origin URL
	originURL, err := url.Parse(origin.URL)
	if err != nil {
		return fmt.Errorf("invalid origin URL: %w", err)
	}

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(originURL)

	// Customize the director to modify the request
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Remove CloudFront signature parameters
		req.URL = RemoveSignatureParams(req.URL)

		// Apply path rewriting if configured
		if origin.StripPrefix != "" {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, origin.StripPrefix)
		}

		// Apply default root object before adding target prefix
		// Check if the path is "/" or empty (both mean root) and if so, rewrite to the configured default
		if req.URL.Path == "" || req.URL.Path == "/" {
			if origin.DefaultRootObject != nil && *origin.DefaultRootObject != "" {
				req.URL.Path = "/" + *origin.DefaultRootObject
			} else if ph.config.Server.DefaultRootObject != "" {
				req.URL.Path = "/" + ph.config.Server.DefaultRootObject
			}
		}

		if origin.TargetPrefix != "" {
			req.URL.Path = origin.TargetPrefix + req.URL.Path
		}

		// Set proper Host header
		req.Host = originURL.Host
		req.Header.Set("Host", originURL.Host)

		// Add CloudFront headers
		req.Header.Set("X-Amz-Cf-Id", generateCloudFrontID())
		req.Header.Set("Via", "1.1 cloudfauxnt")

		// Preserve original headers
		if userAgent := r.Header.Get("User-Agent"); userAgent != "" {
			req.Header.Set("User-Agent", userAgent)
		}
	}

	// Customize response modifier to add CloudFront headers
	proxy.ModifyResponse = func(resp *http.Response) error {
		resp.Header.Set("X-Cache", "Miss from cloudfauxnt")
		resp.Header.Set("X-Amz-Cf-Id", generateCloudFrontID())
		resp.Header.Set("Via", "1.1 cloudfauxnt")
		resp.Header.Set("Server", "CloudFauxnt")
		resp.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
		return nil
	}

	// Handle errors
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		ph.writeCloudFrontError(w, "BadGateway", fmt.Sprintf("Failed to reach origin: %v", err), http.StatusBadGateway)
	}

	// Serve the proxy request
	proxy.ServeHTTP(w, r)
	return nil
}

// writeCloudFrontError writes an error response in CloudFront XML format
func (ph *ProxyHandler) writeCloudFrontError(w http.ResponseWriter, code, message string, status int) {
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("X-Amz-Cf-Id", generateCloudFrontID())
	w.Header().Set("Server", "CloudFauxnt")
	w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
	w.WriteHeader(status)

	errorXML := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>%s</Code>
  <Message>%s</Message>
  <RequestId>%s</RequestId>
</Error>`, code, message, generateCloudFrontID())

	io.WriteString(w, errorXML)
}

// generateCloudFrontID generates a unique CloudFront request ID
func generateCloudFrontID() string {
	id := uuid.New().String()
	return strings.ToUpper(strings.ReplaceAll(id, "-", ""))
}

// HealthHandler handles health check requests
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, `{"status":"healthy","service":"cloudfauxnt"}`)
}

// SetupRouter configures the Chi router with all routes
func SetupRouter(config *Config, validator *SignatureValidator) chi.Router {
	r := chi.NewRouter()

	// Add CORS middleware if enabled
	if config.CORS.Enabled {
		corsMiddleware := NewCORSMiddleware(config.CORS)
		r.Use(corsMiddleware.Handler)
	}

	// Health check endpoint
	r.Get("/health", HealthHandler)

	// Main proxy handler (catch-all)
	proxyHandler := NewProxyHandler(config, validator)
	r.NotFound(proxyHandler.ServeHTTP)

	return r
}
