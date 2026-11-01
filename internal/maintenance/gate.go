package maintenance

import (
	"net/http"
	"strings"
	"sync"
)

type Gate struct{ mutex sync.RWMutex }

func (g *Gate) Exclusive(operation func() error) error {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	return operation()
}

func (g *Gate) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/admin/backups") || strings.HasPrefix(r.URL.Path, "/health/") {
			next.ServeHTTP(w, r)
			return
		}
		g.mutex.RLock()
		defer g.mutex.RUnlock()
		next.ServeHTTP(w, r)
	})
}
