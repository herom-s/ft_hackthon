package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type pgPool interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Ping(ctx context.Context) error
	Close()
}

type PostgresDB struct {
	pool pgPool
}

func NewPostgresDB(databaseURL string) (*PostgresDB, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}

	p, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	db := &PostgresDB{pool: p}
	if err := db.migrate(); err != nil {
		p.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func (db *PostgresDB) CreateUser(user *User) error {
	now := time.Now()
	user.CreatedAt = now
	user.UpdatedAt = now

	_, err := db.pool.Exec(context.Background(),
		`INSERT INTO users (id, username, email, password, gitea_repo_url, gitea_token, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		user.ID, user.Username, user.Email, user.Password, user.GiteaRepoURL, user.GiteaToken, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("user already exists")
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (db *PostgresDB) GetUser(userID string) (*User, error) {
	row := db.pool.QueryRow(context.Background(),
		`SELECT id, username, email, password, gitea_repo_url, gitea_token, created_at, updated_at
		 FROM users WHERE id = $1`, userID,
	)

	user, err := scanUser(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (db *PostgresDB) GetUserByUsername(username string) (*User, error) {
	row := db.pool.QueryRow(context.Background(),
		`SELECT id, username, email, password, gitea_repo_url, gitea_token, created_at, updated_at
		 FROM users WHERE username = $1`, username,
	)

	user, err := scanUser(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return user, nil
}

func (db *PostgresDB) UpdateUser(user *User) error {
	user.UpdatedAt = time.Now()

	_, err := db.pool.Exec(context.Background(),
		`UPDATE users SET username=$1, email=$2, password=$3, gitea_repo_url=$4, gitea_token=$5, updated_at=$6
		 WHERE id=$7`,
		user.Username, user.Email, user.Password, user.GiteaRepoURL, user.GiteaToken, user.UpdatedAt, user.ID,
	)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

func (db *PostgresDB) DeleteUser(userID string) error {
	_, err := db.pool.Exec(context.Background(),
		`DELETE FROM users WHERE id = $1`, userID,
	)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	return nil
}

func (db *PostgresDB) CreateJob(job *Job) error {
	now := time.Now()
	job.CreatedAt = now
	job.UpdatedAt = now
	job.Status = JobStatusQueued

	var resultJSON []byte
	if job.Result != nil {
		var err error
		resultJSON, err = json.Marshal(job.Result)
		if err != nil {
			return fmt.Errorf("marshal job result: %w", err)
		}
	}

	_, err := db.pool.Exec(context.Background(),
		`INSERT INTO jobs (id, user_id, commit_sha, gitea_clone_url, suite, status, message, result, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		job.ID, job.UserID, job.CommitSHA, job.GiteaCloneURL, job.Suite, job.Status, job.Message,
		resultJSON, job.CreatedAt, job.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("create job: %w", err)
	}
	return nil
}

func (db *PostgresDB) GetJob(jobID string) (*Job, error) {
	row := db.pool.QueryRow(context.Background(),
		`SELECT id, user_id, commit_sha, gitea_clone_url, suite, status, message, result, created_at, updated_at
		 FROM jobs WHERE id = $1`, jobID,
	)

	job, err := scanJob(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("job not found")
		}
		return nil, fmt.Errorf("get job: %w", err)
	}
	return job, nil
}

func (db *PostgresDB) GetJobsByUser(userID string) ([]*Job, error) {
	rows, err := db.pool.Query(context.Background(),
		`SELECT id, user_id, commit_sha, gitea_clone_url, suite, status, message, result, created_at, updated_at
		 FROM jobs WHERE user_id = $1 ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("get jobs by user: %w", err)
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (db *PostgresDB) UpdateJob(job *Job) error {
	job.UpdatedAt = time.Now()

	var resultJSON []byte
	if job.Result != nil {
		var err error
		resultJSON, err = json.Marshal(job.Result)
		if err != nil {
			return fmt.Errorf("marshal job result: %w", err)
		}
	}

	_, err := db.pool.Exec(context.Background(),
		`UPDATE jobs SET commit_sha=$1, gitea_clone_url=$2, suite=$3, status=$4, message=$5, result=$6, updated_at=$7
		 WHERE id=$8`,
		job.CommitSHA, job.GiteaCloneURL, job.Suite, job.Status, job.Message, resultJSON, job.UpdatedAt, job.ID,
	)
	if err != nil {
		return fmt.Errorf("update job: %w", err)
	}
	return nil
}

func (db *PostgresDB) GetPendingJobs(limit int) ([]*Job, error) {
	rows, err := db.pool.Query(context.Background(),
		`SELECT id, user_id, commit_sha, gitea_clone_url, suite, status, message, result, created_at, updated_at
		 FROM jobs WHERE status IN ('queued', 'processing')
		 ORDER BY created_at ASC LIMIT $1`, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("get pending jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (db *PostgresDB) GetLeaderboard(suite string, limit int) ([]*LeaderboardEntry, error) {
	rows, err := db.pool.Query(context.Background(),
		`SELECT DISTINCT ON (j.user_id) j.user_id, j.result->>'final_score', j.result->>'benchmark_ms', j.id, COALESCE(u.rating, $3)
		 FROM jobs j
		 LEFT JOIN users u ON u.id = j.user_id
		 WHERE j.suite = $1 AND j.status = 'completed' AND j.result IS NOT NULL
		   AND j.result->>'final_score' IS NOT NULL
		 ORDER BY j.user_id, (j.result->>'final_score')::int DESC
		 LIMIT $2`, suite, limit, DefaultRating,
	)
	if err != nil {
		return nil, fmt.Errorf("get leaderboard: %w", err)
	}
	defer rows.Close()

	var entries []*LeaderboardEntry
	for rows.Next() {
		var userID, scoreStr, bmStr, jobID string
		var rating int
		if err := rows.Scan(&userID, &scoreStr, &bmStr, &jobID, &rating); err != nil {
			return nil, fmt.Errorf("scan leaderboard: %w", err)
		}

		user, _ := db.GetUser(userID)
		username := userID
		if user != nil {
			username = user.Username
		}

		score := 0
		fmt.Sscanf(scoreStr, "%d", &score)
		bm := 0
		fmt.Sscanf(bmStr, "%d", &bm)

		entries = append(entries, &LeaderboardEntry{
			Username:    username,
			Score:       score,
			BenchmarkMs: bm,
			JobID:       jobID,
			Rating:      rating,
		})
	}
	return entries, nil
}

func (db *PostgresDB) SaveResult(jobID string, result *Result) error {
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}

	_, err = db.pool.Exec(context.Background(),
		`UPDATE jobs SET result=$1, status='completed', updated_at=NOW()
		 WHERE id=$2`,
		resultJSON, jobID,
	)
	if err != nil {
		return fmt.Errorf("save result: %w", err)
	}
	return nil
}

func (db *PostgresDB) GetResult(jobID string) (*Result, error) {
	var resultJSON []byte
	err := db.pool.QueryRow(context.Background(),
		`SELECT result FROM jobs WHERE id = $1`, jobID,
	).Scan(&resultJSON)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("job not found")
		}
		return nil, fmt.Errorf("get result: %w", err)
	}

	if resultJSON == nil {
		return nil, fmt.Errorf("no result available")
	}

	var result Result
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}
	return &result, nil
}

func (db *PostgresDB) CreateToken(token, userID string) error {
	_, err := db.pool.Exec(context.Background(),
		`INSERT INTO tokens (token, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		token, userID,
	)
	if err != nil {
		return fmt.Errorf("create token: %w", err)
	}
	return nil
}

func (db *PostgresDB) GetUserIDByToken(token string) (string, error) {
	var userID string
	err := db.pool.QueryRow(context.Background(),
		`SELECT user_id FROM tokens WHERE token = $1`, token,
	).Scan(&userID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("token not found")
		}
		return "", fmt.Errorf("get user by token: %w", err)
	}
	return userID, nil
}

func (db *PostgresDB) DeleteToken(token string) error {
	_, err := db.pool.Exec(context.Background(),
		`DELETE FROM tokens WHERE token = $1`, token,
	)
	if err != nil {
		return fmt.Errorf("delete token: %w", err)
	}
	return nil
}

func (db *PostgresDB) Ping() error {
	return db.pool.Ping(context.Background())
}

func (db *PostgresDB) Close() error {
	db.pool.Close()
	return nil
}

func scanUser(row pgx.Row) (*User, error) {
	var user User
	err := row.Scan(&user.ID, &user.Username, &user.Email, &user.Password,
		&user.GiteaRepoURL, &user.GiteaToken, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func scanJob(row scanner) (*Job, error) {
	var job Job
	var resultJSON []byte

	err := row.Scan(&job.ID, &job.UserID, &job.CommitSHA, &job.GiteaCloneURL,
		&job.Suite, &job.Status, &job.Message, &resultJSON, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if resultJSON != nil {
		var result Result
		if err := json.Unmarshal(resultJSON, &result); err == nil {
			job.Result = &result
		}
	}
	return &job, nil
}

func (db *PostgresDB) ClaimJobs(workerID string, limit int) ([]*Job, error) {
	rows, err := db.pool.Query(context.Background(),
		`UPDATE jobs SET status = 'processing', claimed_by = $1, claimed_at = NOW(), updated_at = NOW()
		 WHERE id IN (
			 SELECT id FROM jobs
			 WHERE status = 'queued'
			 ORDER BY created_at ASC
			 LIMIT $2
			 FOR UPDATE SKIP LOCKED
		 )
		 RETURNING id, user_id, commit_sha, gitea_clone_url, suite, status, message, result, created_at, updated_at`,
		workerID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("claim jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan claimed job: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

func (db *PostgresDB) ReleaseStuckJobs(timeoutMinutes int) (int, error) {
	tag, err := db.pool.Exec(context.Background(),
		`UPDATE jobs SET status = 'queued', claimed_by = '', claimed_at = NULL, updated_at = NOW()
		 WHERE status = 'processing'
		   AND claimed_by != ''
		   AND claimed_at < NOW() - ($1 || ' minutes')::interval`,
		fmt.Sprintf("%d", timeoutMinutes),
	)
	if err != nil {
		return 0, fmt.Errorf("release stuck jobs: %w", err)
	}
	return int(tag.RowsAffected()), nil
}

func (db *PostgresDB) UpdateRating(userID string, newRating int) error {
	_, err := db.pool.Exec(context.Background(),
		`UPDATE users SET rating = $1, updated_at = NOW() WHERE id = $2`,
		newRating, userID,
	)
	if err != nil {
		return fmt.Errorf("update rating: %w", err)
	}
	return nil
}

func (db *PostgresDB) GetSuiteScores(suite string) ([]int, error) {
	rows, err := db.pool.Query(context.Background(),
		`SELECT (j.result->>'final_score')::int
		 FROM jobs j
		 WHERE j.suite = $1 AND j.status = 'completed'
		   AND j.result IS NOT NULL
		   AND j.result->>'final_score' IS NOT NULL`, suite,
	)
	if err != nil {
		return nil, fmt.Errorf("get suite scores: %w", err)
	}
	defer rows.Close()

	var scores []int
	for rows.Next() {
		var score int
		if err := rows.Scan(&score); err != nil {
			return nil, fmt.Errorf("scan score: %w", err)
		}
		scores = append(scores, score)
	}
	return scores, nil
}

func (db *PostgresDB) CheckPlagiarism(suite string) ([]*PlagiarismGroup, error) {
	rows, err := db.pool.Query(context.Background(),
		`SELECT j.result->>'code_checksum', u.username, j.id
		 FROM jobs j
		 LEFT JOIN users u ON u.id = j.user_id
		 WHERE j.suite = $1 AND j.status = 'completed'
		   AND j.result IS NOT NULL
		   AND j.result->>'code_checksum' IS NOT NULL
		   AND j.result->>'code_checksum' != ''
		 ORDER BY j.result->>'code_checksum'`, suite,
	)
	if err != nil {
		return nil, fmt.Errorf("check plagiarism: %w", err)
	}
	defer rows.Close()

	groups := make(map[string]*PlagiarismGroup)
	for rows.Next() {
		var checksum, username, jobID string
		if err := rows.Scan(&checksum, &username, &jobID); err != nil {
			return nil, fmt.Errorf("scan plagiarism: %w", err)
		}
		g, ok := groups[checksum]
		if !ok {
			g = &PlagiarismGroup{Checksum: checksum}
			groups[checksum] = g
		}
		g.Users = append(g.Users, username)
		g.JobIDs = append(g.JobIDs, jobID)
		g.UserCount = len(g.Users)
	}

	result := make([]*PlagiarismGroup, 0, len(groups))
	for _, g := range groups {
		if g.UserCount >= 2 {
			result = append(result, g)
		}
	}
	return result, nil
}

type scanner interface {
	Scan(dest ...interface{}) error
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
