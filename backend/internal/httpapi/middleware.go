package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const maxRateLimitClients = 10_000

type rateLimitEntry struct {
	count   int
	resetAt time.Time
}

type fixedWindowLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	clients map[string]rateLimitEntry
}

func newFixedWindowLimiter(limit int, window time.Duration) *fixedWindowLimiter {
	return &fixedWindowLimiter{limit: limit, window: window, clients: make(map[string]rateLimitEntry)}
}

func (limiter *fixedWindowLimiter) allow(client string, now time.Time) (bool, int, time.Duration) {
	limiter.mu.Lock()
	defer limiter.mu.Unlock()
	entry, exists := limiter.clients[client]
	if !exists || !now.Before(entry.resetAt) {
		entry = rateLimitEntry{resetAt: now.Add(limiter.window)}
	}
	if len(limiter.clients) >= maxRateLimitClients {
		for key, candidate := range limiter.clients {
			if !now.Before(candidate.resetAt) {
				delete(limiter.clients, key)
			}
		}
		if !exists && len(limiter.clients) >= maxRateLimitClients {
			return false, 0, limiter.window
		}
	}
	if entry.count >= limiter.limit {
		return false, 0, entry.resetAt.Sub(now)
	}
	entry.count++
	limiter.clients[client] = entry
	return true, limiter.limit - entry.count, entry.resetAt.Sub(now)
}

func (h *Handler) rateLimit(next http.Handler) http.Handler {
	if h.limiter == nil {
		return next
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if !rateLimitedRequest(request) {
			next.ServeHTTP(writer, request)
			return
		}
		allowed, remaining, resetAfter := h.limiter.allow(clientAddress(request, h.trustProxyHeaders), time.Now())
		writer.Header().Set("X-RateLimit-Limit", strconv.Itoa(h.limiter.limit))
		writer.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		if !allowed {
			retryAfter := int((resetAfter + time.Second - 1) / time.Second)
			if retryAfter < 1 {
				retryAfter = 1
			}
			writer.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			writeError(writer, http.StatusTooManyRequests, "too many requests")
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func rateLimitedRequest(request *http.Request) bool {
	if !strings.HasPrefix(request.URL.Path, "/api/v1/") || strings.HasPrefix(request.URL.Path, "/api/v1/admin/") {
		return false
	}
	return request.Method == http.MethodPost || request.Method == http.MethodPut || request.Method == http.MethodDelete
}

func clientAddress(request *http.Request, trustProxyHeaders bool) string {
	if forwarded := strings.TrimSpace(strings.Split(request.Header.Get("X-Forwarded-For"), ",")[0]); trustProxyHeaders && forwarded != "" {
		return forwarded
	}
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return request.RemoteAddr
}

func (h *Handler) requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requestID := make([]byte, 8)
		if _, err := rand.Read(requestID); err != nil {
			requestID = []byte(strconv.FormatInt(time.Now().UnixNano(), 16))
		}
		writer.Header().Set("X-Request-ID", hex.EncodeToString(requestID))
		next.ServeHTTP(writer, request)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (writer *statusWriter) WriteHeader(status int) {
	if writer.status != 0 {
		return
	}
	writer.status = status
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *statusWriter) Write(body []byte) (int, error) {
	if writer.status == 0 {
		writer.WriteHeader(http.StatusOK)
	}
	written, err := writer.ResponseWriter.Write(body)
	writer.bytes += written
	return written, err
}
