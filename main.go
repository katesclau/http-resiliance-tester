package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
)

type Player struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Position string `json:"position"`
	Country  string `json:"country"`
}

type RateLimiter struct {
	tokens chan struct{}
}

func NewRateLimiter(ratePerSecond int, burst int) *RateLimiter {
	if ratePerSecond <= 0 {
		ratePerSecond = 20
	}
	if burst <= 0 {
		burst = ratePerSecond
	}
	rl := &RateLimiter{
		tokens: make(chan struct{}, burst),
	}
	// Prefill with a burst worth of tokens so the service is immediately usable.
	for i := 0; i < burst; i++ {
		rl.tokens <- struct{}{}
	}
	interval := time.Second / time.Duration(ratePerSecond)
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			select {
			case rl.tokens <- struct{}{}:
			default:
				// Channel full; drop token.
			}
		}
	}()
	return rl
}

func (r *RateLimiter) Allow() bool {
	select {
	case <-r.tokens:
		return true
	default:
		return false
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())

	port := os.Getenv("PORT")
	if strings.TrimSpace(port) == "" {
		port = "8080"
	}
	authToken := os.Getenv("AUTH_TOKEN")
	if strings.TrimSpace(authToken) == "" {
		// Default token for convenience in local runs.
		authToken = "secret-token"
	}

	limiter := NewRateLimiter(20, 20)

	mux := http.NewServeMux()
	playersHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodPost:
			writeJSON(w, http.StatusOK, players())
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		}
	})

	handler := chain(
		requireHeadersAndAuth(authToken),
		withRateLimit(limiter),
		withFlakyBehavior(0.3),
	)(playersHandler)

	mux.Handle("/players", handler)

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("http-resiliance-tester listening on :%s", port)
	log.Printf("Required headers: Content-Type: application/json, Authorization: Bearer %s", authToken)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func players() []Player {
	return []Player{
		{ID: 1, Name: "Jack O'Connell", Position: "Lock", Country: "Ireland"},
		{ID: 2, Name: "Mako Vunipola", Position: "Prop", Country: "England"},
		{ID: 3, Name: "Beauden Barrett", Position: "Fly-half", Country: "New Zealand"},
		{ID: 4, Name: "Antoine Dupont", Position: "Scrum-half", Country: "France"},
		{ID: 5, Name: "Leah Lyons", Position: "Hooker", Country: "Ireland"},
		{ID: 6, Name: "Siya Kolisi", Position: "Flanker", Country: "South Africa"},
		{ID: 7, Name: "Michael Hooper", Position: "Flanker", Country: "Australia"},
		{ID: 8, Name: "Michaela Blyde", Position: "Wing", Country: "New Zealand"},
		{ID: 9, Name: "Emily Scarratt", Position: "Centre", Country: "England"},
		{ID: 10, Name: "Cheslin Kolbe", Position: "Wing", Country: "South Africa"},
	}
}

type middleware func(http.Handler) http.Handler

func chain(middlewares ...middleware) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Apply in reverse so the first middleware wraps outermost.
		for i := len(middlewares) - 1; i >= 0; i-- {
			next = middlewares[i](next)
		}
		return next
	}
}

func withRateLimit(limiter *RateLimiter) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !limiter.Allow() {
				writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded (20 req/s)"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func withFlakyBehavior(failureProbability float64) middleware {
	if failureProbability < 0 {
		failureProbability = 0
	}
	if failureProbability > 1 {
		failureProbability = 1
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if rand.Float64() < failureProbability {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "simulated failure"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func requireHeadersAndAuth(expectedToken string) middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ct := r.Header.Get("Content-Type")
			if ct == "" || !strings.EqualFold(ct, "application/json") {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing or invalid Content-Type (expected application/json)"})
				return
			}
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
				return
			}
			token := strings.TrimPrefix(auth, "Bearer ")
			if strings.TrimSpace(token) != expectedToken {
				writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
