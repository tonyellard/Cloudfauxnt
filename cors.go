package main

import (
	"net/http"
	"strconv"
	"strings"
)

// CORSMiddleware handles CORS preflight and response headers
type CORSMiddleware struct {
	config CORSConfig
}

// NewCORSMiddleware creates a new CORS middleware
func NewCORSMiddleware(config CORSConfig) *CORSMiddleware {
	return &CORSMiddleware{config: config}
}

// Handler wraps an http.Handler with CORS support
func (cm *CORSMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !cm.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")

		// Check if origin is allowed
		if origin != "" && cm.isOriginAllowed(origin) {
			// Set CORS headers
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Vary", "Origin")

			// Handle preflight request
			if r.Method == http.MethodOptions {
				cm.handlePreflight(w, r)
				return
			}
		} else if origin != "" && !cm.isOriginAllowed(origin) {
			// Origin not allowed
			http.Error(w, "Origin not allowed", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handlePreflight handles OPTIONS preflight requests
func (cm *CORSMiddleware) handlePreflight(w http.ResponseWriter, r *http.Request) {
	// Set allowed methods
	methods := strings.Join(cm.config.AllowedMethods, ", ")
	w.Header().Set("Access-Control-Allow-Methods", methods)

	// Set allowed headers
	requestHeaders := r.Header.Get("Access-Control-Request-Headers")
	if requestHeaders != "" {
		if cm.hasWildcard(cm.config.AllowedHeaders) {
			w.Header().Set("Access-Control-Allow-Headers", requestHeaders)
		} else {
			headers := strings.Join(cm.config.AllowedHeaders, ", ")
			w.Header().Set("Access-Control-Allow-Headers", headers)
		}
	}

	// Set max age
	if cm.config.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", strconv.Itoa(cm.config.MaxAge))
	}

	w.WriteHeader(http.StatusNoContent)
}

// isOriginAllowed checks if an origin is in the allowed list
func (cm *CORSMiddleware) isOriginAllowed(origin string) bool {
	for _, allowed := range cm.config.AllowedOrigins {
		if allowed == "*" {
			return true
		}
		if allowed == origin {
			return true
		}
		// Support wildcard subdomains like *.example.com
		if strings.HasPrefix(allowed, "*.") {
			domain := strings.TrimPrefix(allowed, "*")
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}
	return false
}

// hasWildcard checks if a list contains "*"
func (cm *CORSMiddleware) hasWildcard(list []string) bool {
	for _, item := range list {
		if item == "*" {
			return true
		}
	}
	return false
}
