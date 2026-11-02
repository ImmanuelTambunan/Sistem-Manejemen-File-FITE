package middleware

import (
	"sync"
	"time"

	"fite-arsip-api/internal/apperr"
	"fite-arsip-api/internal/httpx"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// FailureLimiter menghitung percobaan login GAGAL per kunci dalam jendela tetap.
type FailureLimiter struct {
	mu     sync.Mutex
	m      map[string]*failure
	max    int
	window time.Duration
}

type failure struct {
	n     int
	reset time.Time
}

func NewFailureLimiter(max int, window time.Duration) *FailureLimiter {
	l := &FailureLimiter{m: map[string]*failure{}, max: max, window: window}
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			l.mu.Lock()
			for k, f := range l.m {
				if time.Now().After(f.reset) {
					delete(l.m, k)
				}
			}
			l.mu.Unlock()
		}
	}()
	return l
}

// Check: diblokir bila kegagalan dalam jendela sudah mencapai batas.
func (l *FailureLimiter) Check(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f := l.m[key]
	if f == nil || time.Now().After(f.reset) || f.n < l.max {
		return true, 0
	}
	return false, time.Until(f.reset)
}

func (l *FailureLimiter) Hit(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if f := l.m[key]; f != nil && now.Before(f.reset) {
		f.n++
		return
	}
	l.m[key] = &failure{n: 1, reset: now.Add(l.window)}
}

func (l *FailureLimiter) Reset(key string) {
	l.mu.Lock()
	delete(l.m, key)
	l.mu.Unlock()
}

// UserLimiter: 60 permintaan/menit per pengguna (ember token, di memori per instans).
type UserLimiter struct {
	mu    sync.Mutex
	m     map[uint]*userEntry
	limit rate.Limit
	burst int
}

type userEntry struct {
	lim  *rate.Limiter
	seen time.Time
}

func NewUserLimiter(perMinute int) *UserLimiter {
	u := &UserLimiter{
		m:     map[uint]*userEntry{},
		limit: rate.Every(time.Minute / time.Duration(perMinute)),
		burst: perMinute,
	}
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			u.mu.Lock()
			for k, e := range u.m {
				if time.Since(e.seen) > 10*time.Minute {
					delete(u.m, k)
				}
			}
			u.mu.Unlock()
		}
	}()
	return u
}

func (u *UserLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		user := CurrentUser(c)
		if user == nil {
			c.Next()
			return
		}
		u.mu.Lock()
		e := u.m[user.ID]
		if e == nil {
			e = &userEntry{lim: rate.NewLimiter(u.limit, u.burst)}
			u.m[user.ID] = e
		}
		e.seen = time.Now()
		ok := e.lim.Allow()
		u.mu.Unlock()
		if !ok {
			httpx.Abort(c, apperr.RateLimited(time.Second))
			return
		}
		c.Next()
	}
}

