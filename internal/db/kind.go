package db

import (
	"net/url"
	"strings"
)

// Kind identifies the configured database backend.
type Kind int

const (
	KindPostgres Kind = iota
	KindSQLite
)

// Detect returns the backend kind for a DATABASE_URL.
// SQLite URLs: sqlite:./data/llms.db, sqlite:///abs/path, sqlite://rel/path
func Detect(databaseURL string) Kind {
	u := strings.TrimSpace(databaseURL)
	lower := strings.ToLower(u)
	switch {
	case strings.HasPrefix(lower, "sqlite:"),
		strings.HasPrefix(lower, "sqlite3:"):
		return KindSQLite
	default:
		return KindPostgres
	}
}

// IsSQLite reports whether databaseURL selects the SQLite backend.
func IsSQLite(databaseURL string) bool {
	return Detect(databaseURL) == KindSQLite
}

// SQLiteFilePath extracts the filesystem path from a sqlite DATABASE_URL.
func SQLiteFilePath(databaseURL string) (string, error) {
	u := strings.TrimSpace(databaseURL)
	lower := strings.ToLower(u)
	switch {
	case strings.HasPrefix(lower, "sqlite://"):
		rest := u[len("sqlite://"):]
		return normalizeSQLitePath(rest), nil
	case strings.HasPrefix(lower, "sqlite3://"):
		rest := u[len("sqlite3://"):]
		return normalizeSQLitePath(rest), nil
	case strings.HasPrefix(lower, "sqlite:"):
		return normalizeSQLitePath(u[len("sqlite:"):]), nil
	case strings.HasPrefix(lower, "sqlite3:"):
		return normalizeSQLitePath(u[len("sqlite3:"):]), nil
	default:
		return "", errNotSQLite
	}
}

func normalizeSQLitePath(rest string) string {
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return "./data/llms.db"
	}
	// Drop query string (migrate options).
	if i := strings.IndexByte(rest, '?'); i >= 0 {
		rest = rest[:i]
	}
	// sqlite:///abs/path → /abs/path; sqlite://./rel → ./rel
	if strings.HasPrefix(rest, "/") && !strings.HasPrefix(rest, "//") {
		return rest
	}
	if parsed, err := url.PathUnescape(rest); err == nil {
		rest = parsed
	}
	return rest
}
