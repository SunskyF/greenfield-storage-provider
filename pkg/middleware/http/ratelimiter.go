package http

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	slimiter "github.com/ulule/limiter/v3"
	smemory "github.com/ulule/limiter/v3/drivers/store/memory"

	"github.com/bnb-chain/greenfield-storage-provider/pkg/log"
)

type RateLimiterCell struct {
	Key        string
	RateLimit  int
	RatePeriod string
}

type IPLimitConfig struct {
	On         bool
	RateLimit  int
	RatePeriod string
}

type RateLimiterConfig struct {
	IPLimitCfg  IPLimitConfig
	PathPattern []RateLimiterCell
	HostPattern []RateLimiterCell
	APILimits   []RateLimiterCell
}

type MemoryLimiterConfig struct {
	RateLimit  int    // rate
	RatePeriod string // per period
}

type APILimiterConfig struct {
	IPLimitCfg  IPLimitConfig
	PathPattern map[string]MemoryLimiterConfig
	APILimits   map[string]MemoryLimiterConfig // routePrefix-apiName  =>  limit config
	HostPattern map[string]MemoryLimiterConfig
}

type apiLimiter struct {
	store      slimiter.Store
	limiterMap sync.Map
	cfg        APILimiterConfig
}

var limiter *apiLimiter

func NewAPILimiter(cfg *APILimiterConfig) error {
	localStore := smemory.NewStoreWithOptions(slimiter.StoreOptions{
		Prefix:          "sp_api_rate_limiter",
		CleanUpInterval: 5 * time.Second,
	})
	limiter = &apiLimiter{
		store: localStore,
		cfg: APILimiterConfig{
			APILimits:   make(map[string]MemoryLimiterConfig),
			PathPattern: make(map[string]MemoryLimiterConfig),
			HostPattern: make(map[string]MemoryLimiterConfig),
			IPLimitCfg:  cfg.IPLimitCfg,
		},
	}

	var err error
	var rate slimiter.Rate

	for k, v := range cfg.PathPattern {
		limiter.cfg.PathPattern[strings.ToLower(k)] = v
	}

	for k, v := range cfg.HostPattern {
		limiter.cfg.HostPattern[strings.ToLower(k)] = v
	}

	for k, v := range cfg.APILimits {
		rate, err = slimiter.NewRateFromFormatted(fmt.Sprintf("%d-%s", v.RateLimit, v.RatePeriod))
		if err != nil {
			return err
		}

		limiter.limiterMap.Store(strings.ToLower(k), slimiter.New(localStore, rate))
	}

	return nil
}

func (a *apiLimiter) findLimiter(host, path, key string) *slimiter.Limiter {
	newLimiter, ok := a.limiterMap.Load(key)
	if ok {
		return newLimiter.(*slimiter.Limiter)
	}

	for p, l := range a.cfg.HostPattern {
		if regexp.MustCompile(p).MatchString(host) {
			rate, err := slimiter.NewRateFromFormatted(fmt.Sprintf("%d-%s", l.RateLimit, l.RatePeriod))
			if err != nil {
				log.Errorw("failed to new rate from formatted", "err", err)
				continue
			}
			newLimiter = slimiter.New(a.store, rate)
			return newLimiter.(*slimiter.Limiter)
		}
	}

	for p, l := range a.cfg.PathPattern {
		if regexp.MustCompile(p).MatchString(path) {
			rate, err := slimiter.NewRateFromFormatted(fmt.Sprintf("%d-%s", l.RateLimit, l.RatePeriod))
			if err != nil {
				log.Errorw("failed to new rate from formatted", "err", err)
				continue
			}
			newLimiter = slimiter.New(a.store, rate)
			return newLimiter.(*slimiter.Limiter)
		}
	}

	return nil
}

func (t *apiLimiter) Allow(ctx context.Context, r *http.Request) bool {
	path := strings.ToLower(r.RequestURI)
	host := r.Host
	key := host + "-" + path
	key = strings.ToLower(key)

	l := t.findLimiter(host, path, key)
	if l == nil {
		return true
	}

	limiterCtx, err := t.store.Increment(ctx, key, 1, l.Rate)
	if err != nil {
		return true
	}

	if limiterCtx.Reached {
		return false
	}
	return true
}

func (t *apiLimiter) HTTPAllow(ctx context.Context, r *http.Request) bool {
	if !t.cfg.IPLimitCfg.On {
		return true
	}
	ipStr := GetIP(r)
	key := "ip_" + ipStr

	rate, err := slimiter.NewRateFromFormatted(fmt.Sprintf("%d-%s", t.cfg.IPLimitCfg.RateLimit, t.cfg.IPLimitCfg.RatePeriod))
	if err != nil {
		log.Errorw("failed to new rate from formatted", "err", err)
		return true
	}
	limiterCtx, err := t.store.Increment(ctx, key, 1, rate)
	if err != nil {
		return true
	}

	if limiterCtx.Reached {
		return false
	}
	return true
}

func Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow(context.Background(), r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		if !limiter.HTTPAllow(context.Background(), r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// GetIP gets a requests IP address by reading off the forwarded-for
// header (for proxies) and falls back to use the remote address.
func GetIP(r *http.Request) string {
	forwarded := r.Header.Get("X-FORWARDED-FOR")
	if forwarded != "" {
		return forwarded
	}
	return r.RemoteAddr
}
