package main

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Visitors struct {
	limiter  *rate.Limiter
	mutex    sync.Mutex
	lastSeen time.Time
}

func (v *Visitors) Seen(time time.Time) {
	// This lock can be removed  since we only need minute level granularity
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.lastSeen = time
}

func PerClientRateLimiter(next func(w http.ResponseWriter, r *http.Request)) http.Handler {
	var (
		mutex    sync.RWMutex
		visitors = make(map[string]*Visitors)
	)

	go func() {
		for range time.Tick(time.Minute) {
			for ip, visitor := range visitors {
				if time.Since(visitor.lastSeen) > 3*time.Minute {
					mutex.Lock()
					delete(visitors, ip)
					mutex.Unlock()
				}
			}
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// get the client ip address
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(
				w,
				"Internal server error",
				http.StatusInternalServerError,
			)
			return
		}
		mutex.RLock()
		visitor, found := visitors[ip]
		mutex.RUnlock()
		if !found {
			visitor = &Visitors{limiter: rate.NewLimiter(2, 4)}
			mutex.Lock()
			visitors[ip] = visitor
			mutex.Unlock()
		}
		visitor.Seen(time.Now())
		if !visitor.limiter.Allow() {
			message := Message{
				Status: "Request failed",
				Data:   "Too many request!, please try again later",
			}
			w.WriteHeader(http.StatusTooManyRequests)
			err := json.NewEncoder(w).Encode(&message)
			if err != nil {
				return
			}
		}
		next(w, r)
	})

}
