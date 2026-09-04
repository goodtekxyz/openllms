package store_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/goodtekxyz/openllms/internal/db"
	"github.com/goodtekxyz/openllms/internal/store"
)

func TestSQLiteBootstrapLookupRouteAccount(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "llms.db")
	url := "sqlite:" + dbPath

	if err := db.Migrate(url); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqlDB, err := db.ConnectSQLite(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	st := store.NewSQLite(sqlDB)
	ctx := context.Background()

	projectID, key, err := st.Bootstrap(ctx, "oss-user", "default", "cli")
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if key == "" {
		t.Fatal("empty api key")
	}

	ac, err := st.LookupByPlaintext(ctx, key)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if ac.ProjectID != projectID {
		t.Fatalf("project mismatch: %s vs %s", ac.ProjectID, projectID)
	}

	rt, err := st.CreateRoute(ctx, projectID, "default", "sequential", "gpt-4o-mini", []byte(`{}`))
	if err != nil {
		t.Fatalf("create route: %v", err)
	}

	acc, err := st.CreateAccount(ctx, projectID, "openai", "main", "api_key", "/projects/"+projectID.String()+"/openai/main", "https://api.openai.com/v1")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if err := st.AttachAccount(ctx, rt.ID, acc.ID, 0, 1); err != nil {
		t.Fatalf("attach: %v", err)
	}

	pool, err := st.ListRoutePool(ctx, rt.ID)
	if err != nil {
		t.Fatalf("list pool: %v", err)
	}
	if len(pool) != 1 || pool[0].ID != acc.ID {
		t.Fatalf("pool=%v", pool)
	}

	keyID := ac.KeyID
	if err := st.InsertUsage(ctx, projectID, &rt.ID, &acc.ID, &keyID, "gpt-4o-mini", 200, 12, 3, 5, ""); err != nil {
		t.Fatalf("usage: %v", err)
	}

	n, err := st.RefreshQuotasFromUsage(ctx)
	if err != nil {
		t.Fatalf("refresh quotas: %v", err)
	}
	if n < 1 {
		t.Fatalf("expected >=1 quota row, got %d", n)
	}

	users, err := st.ListAdminUsers(ctx)
	if err != nil {
		t.Fatalf("admin users: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("users=%d", len(users))
	}

	if err := st.Ping(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}
	if st.Dialect() != store.DialectSQLite {
		t.Fatalf("dialect=%v", st.Dialect())
	}
}
