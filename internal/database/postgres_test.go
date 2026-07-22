package database

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type mockRow struct {
	scan func(dest ...any) error
}

func (m *mockRow) Scan(dest ...any) error { return m.scan(dest...) }

type mockRows struct {
	data   []func(dest ...any) error
	idx    int
	closed bool
	err    error
}

func newMockRows(data []func(dest ...any) error) *mockRows {
	return &mockRows{data: data, idx: -1}
}

func (m *mockRows) Close()                   { m.closed = true }
func (m *mockRows) Err() error               { return m.err }
func (m *mockRows) CommandTag() pgconn.CommandTag { return pgconn.NewCommandTag("") }
func (m *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *mockRows) Next() bool {
	if m.err != nil {
		return false
	}
	m.idx++
	return m.idx < len(m.data)
}
func (m *mockRows) Scan(dest ...any) error {
	if m.idx < 0 || m.idx >= len(m.data) {
		return pgx.ErrNoRows
	}
	return m.data[m.idx](dest...)
}
func (m *mockRows) Values() ([]any, error)     { return nil, nil }
func (m *mockRows) RawValues() [][]byte        { return nil }
func (m *mockRows) Conn() *pgx.Conn            { return nil }

type mockPool struct {
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) pgx.Row
	queryFn    func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	pingFn     func(ctx context.Context) error
	closeFn    func()
}

func (m *mockPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.execFn != nil {
		return m.execFn(ctx, sql, args...)
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}
func (m *mockPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if m.queryRowFn != nil {
		return m.queryRowFn(ctx, sql, args...)
	}
	return &mockRow{scan: func(dest ...any) error { return pgx.ErrNoRows }}
}
func (m *mockPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if m.queryFn != nil {
		return m.queryFn(ctx, sql, args...)
	}
	return newMockRows(nil), nil
}
func (m *mockPool) Ping(ctx context.Context) error {
	if m.pingFn != nil {
		return m.pingFn(ctx)
	}
	return nil
}
func (m *mockPool) Close() {
	if m.closeFn != nil {
		m.closeFn()
	}
}

func TestNewPostgresDB_InvalidURL(t *testing.T) {
	_, err := NewPostgresDB("not-a-valid-url")
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestPostgresDB_CreateUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("INSERT 0 1"), nil
			},
		}}
		err := db.CreateUser(&User{ID: "u1", Username: "alice", Email: "a@b.com", Password: "pw"})
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("unique violation", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("INSERT 0 1"), &pgconn.PgError{Code: "23505"}
			},
		}}
		err := db.CreateUser(&User{ID: "u1", Username: "alice"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("other error", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("INSERT 0 1"), errors.New("connection failed")
			},
		}}
		err := db.CreateUser(&User{ID: "u1"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPostgresDB_GetUser(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		now := time.Now()
		db := &PostgresDB{pool: &mockPool{
			queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return &mockRow{scan: func(dest ...any) error {
					*(dest[0].(*string)) = "u1"
					*(dest[1].(*string)) = "alice"
					*(dest[2].(*string)) = "a@b.com"
					*(dest[3].(*string)) = "pw"
					*(dest[4].(*string)) = ""
					*(dest[5].(*string)) = ""
					*(dest[6].(*time.Time)) = now
					*(dest[7].(*time.Time)) = now
					return nil
				}}
			},
		}}
		user, err := db.GetUser("u1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if user.Username != "alice" {
			t.Errorf("expected alice, got %s", user.Username)
		}
	})

	t.Run("not found", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return &mockRow{scan: func(dest ...any) error { return pgx.ErrNoRows }}
			},
		}}
		_, err := db.GetUser("u999")
		if err == nil {
			t.Fatal("expected error for not found")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return &mockRow{scan: func(dest ...any) error { return errors.New("scan failed") }}
			},
		}}
		_, err := db.GetUser("u1")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPostgresDB_GetUserByUsername(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		now := time.Now()
		db := &PostgresDB{pool: &mockPool{
			queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return &mockRow{scan: func(dest ...any) error {
					*(dest[0].(*string)) = "u1"
					*(dest[1].(*string)) = "alice"
					*(dest[2].(*string)) = "a@b.com"
					*(dest[3].(*string)) = "pw"
					*(dest[4].(*string)) = ""
					*(dest[5].(*string)) = ""
					*(dest[6].(*time.Time)) = now
					*(dest[7].(*time.Time)) = now
					return nil
				}}
			},
		}}
		user, err := db.GetUserByUsername("alice")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if user.ID != "u1" {
			t.Errorf("expected u1, got %s", user.ID)
		}
	})

	t.Run("not found", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return &mockRow{scan: func(dest ...any) error { return pgx.ErrNoRows }}
			},
		}}
		_, err := db.GetUserByUsername("bob")
		if err == nil {
			t.Fatal("expected error for not found")
		}
	})
}

func TestPostgresDB_UpdateUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("UPDATE 1"), nil
			},
		}}
		err := db.UpdateUser(&User{ID: "u1", Username: "alice_updated"})
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("UPDATE 0"), errors.New("update failed")
			},
		}}
		err := db.UpdateUser(&User{ID: "u1"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPostgresDB_DeleteUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("DELETE 1"), nil
			},
		}}
		err := db.DeleteUser("u1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("DELETE 0"), errors.New("delete failed")
			},
		}}
		err := db.DeleteUser("u1")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPostgresDB_CreateJob(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		executed := false
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				executed = true
				return pgconn.NewCommandTag("INSERT 0 1"), nil
			},
		}}
		err := db.CreateJob(&Job{ID: "j1", UserID: "u1"})
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if !executed {
			t.Error("expected exec to be called")
		}
	})

	t.Run("with result", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("INSERT 0 1"), nil
			},
		}}
		job := &Job{ID: "j2", UserID: "u1", Result: &Result{FinalScore: 90}}
		err := db.CreateJob(job)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("INSERT 0 0"), errors.New("insert failed")
			},
		}}
		err := db.CreateJob(&Job{ID: "j1", UserID: "u1"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPostgresDB_GetJob(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		now := time.Now()
		db := &PostgresDB{pool: &mockPool{
			queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return &mockRow{scan: func(dest ...any) error {
					*(dest[0].(*string)) = "j1"
					*(dest[1].(*string)) = "u1"
					*(dest[2].(*string)) = "abc"
					*(dest[3].(*string)) = "http://clone"
					*(dest[4].(*string)) = "suite1"
					*(dest[5].(*string)) = "completed"
					*(dest[6].(*string)) = "done"
					*(dest[7].(*[]byte)) = []byte(`{"final_score": 90}`)
					*(dest[8].(*time.Time)) = now
					*(dest[9].(*time.Time)) = now
					return nil
				}}
			},
		}}
		job, err := db.GetJob("j1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if job.Suite != "suite1" {
			t.Errorf("expected suite1, got %s", job.Suite)
		}
		if job.Result == nil || job.Result.FinalScore != 90 {
			t.Errorf("expected result final_score 90")
		}
	})

	t.Run("not found", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return &mockRow{scan: func(dest ...any) error { return pgx.ErrNoRows }}
			},
		}}
		_, err := db.GetJob("j999")
		if err == nil {
			t.Fatal("expected error for not found")
		}
	})

	t.Run("scan error", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return &mockRow{scan: func(dest ...any) error { return errors.New("scan failed") }}
			},
		}}
		_, err := db.GetJob("j1")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPostgresDB_GetJobsByUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		now := time.Now()
		db := &PostgresDB{pool: &mockPool{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return newMockRows([]func(dest ...any) error{
					func(dest ...any) error {
						*(dest[0].(*string)) = "j1"
						*(dest[1].(*string)) = "u1"
						*(dest[2].(*string)) = "abc"
						*(dest[3].(*string)) = ""
						*(dest[4].(*string)) = "s1"
						*(dest[5].(*string)) = "completed"
						*(dest[6].(*string)) = "ok"
						*(dest[7].(*[]byte)) = []byte("{}")
						*(dest[8].(*time.Time)) = now
						*(dest[9].(*time.Time)) = now
						return nil
					},
					func(dest ...any) error {
						*(dest[0].(*string)) = "j2"
						*(dest[1].(*string)) = "u1"
						*(dest[2].(*string)) = "def"
						*(dest[3].(*string)) = ""
						*(dest[4].(*string)) = "s1"
						*(dest[5].(*string)) = "failed"
						*(dest[6].(*string)) = "err"
						*(dest[7].(*[]byte)) = nil
						*(dest[8].(*time.Time)) = now
						*(dest[9].(*time.Time)) = now
						return nil
					},
				}), nil
			},
		}}
		jobs, err := db.GetJobsByUser("u1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(jobs) != 2 {
			t.Fatalf("expected 2 jobs, got %d", len(jobs))
		}
	})

	t.Run("query error", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return nil, errors.New("query failed")
			},
		}}
		_, err := db.GetJobsByUser("u1")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPostgresDB_UpdateJob(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("UPDATE 1"), nil
			},
		}}
		err := db.UpdateJob(&Job{ID: "j1"})
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("UPDATE 0"), errors.New("update failed")
			},
		}}
		err := db.UpdateJob(&Job{ID: "j1"})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPostgresDB_GetPendingJobs(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		now := time.Now()
		db := &PostgresDB{pool: &mockPool{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return newMockRows([]func(dest ...any) error{
					func(dest ...any) error {
						*(dest[0].(*string)) = "j1"
						*(dest[1].(*string)) = "u1"
						*(dest[2].(*string)) = "abc"
						*(dest[3].(*string)) = ""
						*(dest[4].(*string)) = "s1"
						*(dest[5].(*string)) = "queued"
						*(dest[6].(*string)) = ""
						*(dest[7].(*[]byte)) = nil
						*(dest[8].(*time.Time)) = now
						*(dest[9].(*time.Time)) = now
						return nil
					},
				}), nil
			},
		}}
		jobs, err := db.GetPendingJobs(10)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(jobs) != 1 {
			t.Fatalf("expected 1 job, got %d", len(jobs))
		}
	})

	t.Run("query error", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return nil, errors.New("query failed")
			},
		}}
		_, err := db.GetPendingJobs(10)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPostgresDB_GetLeaderboard(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := &PostgresDB{
			pool: &mockPool{
				queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return newMockRows([]func(dest ...any) error{
						func(dest ...any) error {
							*(dest[0].(*string)) = "u1"
							*(dest[1].(*string)) = "90"
							*(dest[2].(*string)) = "100"
							*(dest[3].(*string)) = "j1"
							*(dest[4].(*int)) = 1200
							return nil
						},
						func(dest ...any) error {
							*(dest[0].(*string)) = "u2"
							*(dest[1].(*string)) = "80"
							*(dest[2].(*string)) = "200"
							*(dest[3].(*string)) = "j2"
							*(dest[4].(*int)) = 1200
							return nil
						},
					}), nil
				},
				queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &mockRow{scan: func(dest ...any) error {
						return errors.New("no user found")
					}}
				},
			},
		}
		entries, err := db.GetLeaderboard("suite1", 10)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(entries) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(entries))
		}
		if entries[0].Score != 90 {
			t.Errorf("expected 90, got %d", entries[0].Score)
		}
	})

	t.Run("query error", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return nil, errors.New("query failed")
			},
		}}
		_, err := db.GetLeaderboard("suite1", 10)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPostgresDB_SaveResult(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("UPDATE 1"), nil
			},
		}}
		err := db.SaveResult("j1", &Result{FinalScore: 90})
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("UPDATE 0"), errors.New("update failed")
			},
		}}
		err := db.SaveResult("j1", &Result{})
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPostgresDB_GetResult(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return &mockRow{scan: func(dest ...any) error {
					*(dest[0].(*[]byte)) = []byte(`{"final_score": 90}`)
					return nil
				}}
			},
		}}
		result, err := db.GetResult("j1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if result.FinalScore != 90 {
			t.Errorf("expected 90, got %d", result.FinalScore)
		}
	})

	t.Run("not found", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return &mockRow{scan: func(dest ...any) error { return pgx.ErrNoRows }}
			},
		}}
		_, err := db.GetResult("j999")
		if err == nil {
			t.Fatal("expected error for not found")
		}
	})

	t.Run("nil result", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return &mockRow{scan: func(dest ...any) error {
					*(dest[0].(*[]byte)) = nil
					return nil
				}}
			},
		}}
		_, err := db.GetResult("j1")
		if err == nil {
			t.Fatal("expected error for nil result")
		}
	})
}

func TestPostgresDB_TokenOperations(t *testing.T) {
	t.Run("CreateToken success", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("INSERT 0 1"), nil
			},
		}}
		err := db.CreateToken("tok1", "u1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("CreateToken error", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("INSERT 0 0"), errors.New("insert failed")
			},
		}}
		err := db.CreateToken("tok1", "u1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("GetUserIDByToken found", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return &mockRow{scan: func(dest ...any) error {
					*(dest[0].(*string)) = "u1"
					return nil
				}}
			},
		}}
		uid, err := db.GetUserIDByToken("tok1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if uid != "u1" {
			t.Errorf("expected u1, got %s", uid)
		}
	})

	t.Run("GetUserIDByToken not found", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
				return &mockRow{scan: func(dest ...any) error { return pgx.ErrNoRows }}
			},
		}}
		_, err := db.GetUserIDByToken("bad")
		if err == nil {
			t.Fatal("expected error for not found")
		}
	})

	t.Run("DeleteToken success", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("DELETE 1"), nil
			},
		}}
		err := db.DeleteToken("tok1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("DeleteToken error", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("DELETE 0"), errors.New("delete failed")
			},
		}}
		err := db.DeleteToken("tok1")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPostgresDB_Ping(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			pingFn: func(ctx context.Context) error { return nil },
		}}
		err := db.Ping()
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			pingFn: func(ctx context.Context) error { return errors.New("ping failed") },
		}}
		err := db.Ping()
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPostgresDB_Close(t *testing.T) {
	closed := false
	db := &PostgresDB{pool: &mockPool{
		closeFn: func() { closed = true },
	}}
	err := db.Close()
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if !closed {
		t.Error("expected pool.Close to be called")
	}
}

func TestPostgresDB_UpdateRating(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("UPDATE 1"), nil
			},
		}}
		err := db.UpdateRating("u1", 1500)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("UPDATE 0"), errors.New("update failed")
			},
		}}
		err := db.UpdateRating("u1", 1500)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPostgresDB_GetSuiteScores(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return newMockRows([]func(dest ...any) error{
					func(dest ...any) error {
						*(dest[0].(*int)) = 90
						return nil
					},
					func(dest ...any) error {
						*(dest[0].(*int)) = 80
						return nil
					},
				}), nil
			},
		}}
		scores, err := db.GetSuiteScores("suite1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(scores) != 2 {
			t.Fatalf("expected 2 scores, got %d", len(scores))
		}
	})

	t.Run("query error", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return nil, errors.New("query failed")
			},
		}}
		_, err := db.GetSuiteScores("suite1")
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPostgresDB_CheckPlagiarism(t *testing.T) {
	t.Run("detects duplicates", func(t *testing.T) {
		db := &PostgresDB{
			pool: &mockPool{
				queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
					return newMockRows([]func(dest ...any) error{
						func(dest ...any) error {
							*(dest[0].(*string)) = "cs1"
							*(dest[1].(*string)) = "alice"
							*(dest[2].(*string)) = "j1"
							return nil
						},
						func(dest ...any) error {
							*(dest[0].(*string)) = "cs1"
							*(dest[1].(*string)) = "bob"
							*(dest[2].(*string)) = "j2"
							return nil
						},
						func(dest ...any) error {
							*(dest[0].(*string)) = "cs2"
							*(dest[1].(*string)) = "charlie"
							*(dest[2].(*string)) = "j3"
							return nil
						},
					}), nil
				},
				queryRowFn: func(ctx context.Context, sql string, args ...any) pgx.Row {
					return &mockRow{scan: func(dest ...any) error { return pgx.ErrNoRows }}
				},
			},
		}
		groups, err := db.CheckPlagiarism("suite1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(groups))
		}
		if groups[0].UserCount != 2 {
			t.Errorf("expected 2 users, got %d", groups[0].UserCount)
		}
	})

	t.Run("query error", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return nil, errors.New("query failed")
			},
		}}
		_, err := db.CheckPlagiarism("suite1")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("no duplicates", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return newMockRows([]func(dest ...any) error{
					func(dest ...any) error {
						*(dest[0].(*string)) = "cs1"
						*(dest[1].(*string)) = "alice"
						*(dest[2].(*string)) = "j1"
						return nil
					},
					func(dest ...any) error {
						*(dest[0].(*string)) = "cs2"
						*(dest[1].(*string)) = "bob"
						*(dest[2].(*string)) = "j2"
						return nil
					},
				}), nil
			},
		}}
		groups, err := db.CheckPlagiarism("suite1")
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(groups) != 0 {
			t.Errorf("expected 0 groups, got %d", len(groups))
		}
	})
}

func TestMigrate(t *testing.T) {
	t.Run("creates schema_migrations table", func(t *testing.T) {
		var created bool
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				if sql == "CREATE TABLE IF NOT EXISTS schema_migrations (\n\t\t\tid INT PRIMARY KEY,\n\t\t\tdescription TEXT NOT NULL,\n\t\t\tapplied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()\n\t\t)" {
					created = true
				}
				return pgconn.NewCommandTag("CREATE TABLE"), nil
			},
		}}
		err := db.migrate()
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if !created {
			t.Error("expected schema_migrations table creation")
		}
	})

	t.Run("applies all pending migrations", func(t *testing.T) {
		var execCount int
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				execCount++
				return pgconn.NewCommandTag("INSERT 0 1"), nil
			},
		}}
		err := db.migrate()
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		// exec count = 1 (ensure table) + 6 (migrations SQL) + 6 (record each) = 13
		if execCount != 13 {
			t.Errorf("expected 13 exec calls, got %d", execCount)
		}
	})

	t.Run("skips already applied migrations", func(t *testing.T) {
		var execCount int
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				execCount++
				return pgconn.NewCommandTag("INSERT 0 1"), nil
			},
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return newMockRows([]func(dest ...any) error{
					func(dest ...any) error {
						*(dest[0].(*int)) = 1
						return nil
					},
					func(dest ...any) error {
						*(dest[0].(*int)) = 2
						return nil
					},
					func(dest ...any) error {
						*(dest[0].(*int)) = 3
						return nil
					},
				}), nil
			},
		}}
		err := db.migrate()
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		// exec count = 1 (ensure table) + 3 (migrations 4+5+6 SQL) + 3 (record 4+5+6) = 7
		if execCount != 7 {
			t.Errorf("expected 7 exec calls, got %d", execCount)
		}
	})

	t.Run("fails on migration error", func(t *testing.T) {
		var callCount int
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				callCount++
				if callCount == 3 {
					return pgconn.NewCommandTag(""), errors.New("syntax error")
				}
				return pgconn.NewCommandTag("INSERT 0 1"), nil
			},
		}}
		err := db.migrate()
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPostgresDB_ClaimJobs(t *testing.T) {
	t.Run("claims queued jobs", func(t *testing.T) {
		var executedSQL string
		db := &PostgresDB{pool: &mockPool{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				executedSQL = sql
				return newMockRows([]func(dest ...any) error{
					func(dest ...any) error {
						*(dest[0].(*string)) = "j1"
						*(dest[1].(*string)) = "u1"
						*(dest[2].(*string)) = "abc123"
						*(dest[3].(*string)) = "http://clone"
						*(dest[4].(*string)) = "libft"
						*(dest[5].(*string)) = "processing"
						*(dest[6].(*string)) = ""
						*(dest[7].(*[]byte)) = nil
						*(dest[8].(*time.Time)) = time.Now()
						*(dest[9].(*time.Time)) = time.Now()
						return nil
					},
				}), nil
			},
		}}
		jobs, err := db.ClaimJobs("worker-1", 5)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(jobs) != 1 {
			t.Fatalf("expected 1 job, got %d", len(jobs))
		}
		if jobs[0].ID != "j1" {
			t.Errorf("expected j1, got %s", jobs[0].ID)
		}
		if !strings.Contains(executedSQL, "FOR UPDATE SKIP LOCKED") {
			t.Error("expected SKIP LOCKED in query")
		}
	})

	t.Run("returns empty when no queued jobs", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return newMockRows(nil), nil
			},
		}}
		jobs, err := db.ClaimJobs("worker-2", 5)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if len(jobs) != 0 {
			t.Errorf("expected 0 jobs, got %d", len(jobs))
		}
	})

	t.Run("query error", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return nil, errors.New("connection failed")
			},
		}}
		_, err := db.ClaimJobs("worker-3", 5)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestPostgresDB_ReleaseStuckJobs(t *testing.T) {
	t.Run("releases stuck jobs", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("UPDATE 2"), nil
			},
		}}
		count, err := db.ReleaseStuckJobs(10)
		if err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
		if count != 2 {
			t.Errorf("expected 2, got %d", count)
		}
	})

	t.Run("error", func(t *testing.T) {
		db := &PostgresDB{pool: &mockPool{
			execFn: func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				return pgconn.NewCommandTag("UPDATE 0"), errors.New("release failed")
			},
		}}
		_, err := db.ReleaseStuckJobs(10)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}
