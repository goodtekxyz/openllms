package db

import "testing"

func TestDetectSQLite(t *testing.T) {
	cases := []struct {
		url  string
		want Kind
	}{
		{"postgres://u:p@localhost/db", KindPostgres},
		{"sqlite:./data/llms.db", KindSQLite},
		{"sqlite:///tmp/x.db", KindSQLite},
		{"sqlite3:./x.db", KindSQLite},
	}
	for _, tc := range cases {
		if got := Detect(tc.url); got != tc.want {
			t.Fatalf("%s: got %v want %v", tc.url, got, tc.want)
		}
	}
	path, err := SQLiteFilePath("sqlite:./data/llms.db")
	if err != nil || path != "./data/llms.db" {
		t.Fatalf("path=%q err=%v", path, err)
	}
}
