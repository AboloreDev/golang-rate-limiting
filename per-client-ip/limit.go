package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func (v *Visitor) Allow(now time.Time) bool {
	v.lastSeen = now
	return v.limiter.Allow()
}


type VisitorLimiter struct {
	visitors map[string]*Visitor
	ctx context.Context
	mutex    sync.RWMutex
}

func newVisitorLimiter(ctx context.Context) *VisitorLimiter {
	limiter := &VisitorLimiter{
		visitors: make(map[string]*Visitor),
		ctx: ctx,
	}

	go limiter.runEvictor()
	return limiter
}

func (vl *VisitorLimiter) GetVisitor(fingerprint string, fallback *Visitor) *Visitor {
	vl.mutex.RLock()
	visitor, found := vl.visitors[fingerprint]
	vl.mutex.RUnlock()
	if !found {
		vl.mutex.Lock()
		vl.visitors[fingerprint] = fallback
		defer vl.mutex.Unlock()
	}
	return visitor
}

func (vl *VisitorLimiter) runEvictor() {
	for {
		select {
		case <- time.Tick(time.Minute):
			for ip, visitor := range vl.visitors {
			if time.Since(visitor.lastSeen) > 3*time.Minute {
				vl.mutex.Lock()
				delete(vl.visitors, ip)
				vl.mutex.Unlock()
			}
		}
		case <-vl.ctx.Done():
			return
		}
	}
}

func PerClientRateLimiter(next func(w http.ResponseWriter, r *http.Request)) http.Handler {
	visitorLimiter := newVisitorLimiter(context.Background())

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
		visitor := visitorLimiter.GetVisitor(ip, &Visitor{limiter: rate.NewLimiter(2, 4)})
		if !visitor.Allow(time.Now()) {
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
