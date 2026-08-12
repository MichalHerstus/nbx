package datasource

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

// DefaultConnectionTTL is the default time a cached external connection
// is kept open after its last use before being closed and reaped.
const DefaultConnectionTTL = 5 * time.Minute

// connEntry holds a single cached external database connection.
type connEntry struct {
	db       *dbx.DB
	lastUsed time.Time
}

// Registry manages the external (mysql/postgres/mssql) data source
// connections used by the records API.
//
// Connections are pooled by their resolved config (driver + DSN) and are
// lazily reaped after a period of inactivity.
type Registry struct {
	mu    sync.Mutex
	conns map[string]*connEntry
	ttl   time.Duration
	now   func() time.Time
}

// NewRegistry initializes a new empty datasource connection Registry.
func NewRegistry() *Registry {
	return &Registry{
		conns: make(map[string]*connEntry),
		ttl:   DefaultConnectionTTL,
		now:   time.Now,
	}
}

// Get opens (or reuses) a cached connection for the provided SQL datasource
// config and resolved credential.
func (r *Registry) Get(ds core.DataSource, cred core.Credential) (*dbx.DB, error) {
	driver, dsn, err := buildDSN(ds, cred)
	if err != nil {
		return nil, err
	}

	builderName := builderNameFor(ds.Type)
	key := cacheKey(builderName, dsn)

	r.mu.Lock()
	if entry, ok := r.conns[key]; ok {
		entry.lastUsed = r.now()
		db := entry.db
		r.mu.Unlock()
		return db, nil
	}
	r.mu.Unlock()

	sqlDB, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	db := dbx.NewFromDB(sqlDB, builderName)

	r.mu.Lock()
	if entry, ok := r.conns[key]; ok {
		// another goroutine opened the same connection first; close ours
		r.mu.Unlock()
		_ = db.Close()
		return entry.db, nil
	}

	r.conns[key] = &connEntry{db: db, lastUsed: r.now()}
	r.mu.Unlock()

	return db, nil
}

// Close closes all cached connections and clears the registry.
func (r *Registry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for key, entry := range r.conns {
		_ = entry.db.Close()
		delete(r.conns, key)
	}
}

// Reap closes connections that have not been used for more than the
// configured TTL.
func (r *Registry) Reap() {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := r.now().Add(-r.ttl)
	for key, entry := range r.conns {
		if entry.lastUsed.Before(cutoff) {
			_ = entry.db.Close()
			delete(r.conns, key)
		}
	}
}

// Len returns the number of cached connections.
func (r *Registry) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()

	return len(r.conns)
}

func cacheKey(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
