package db

import (
	"context"
	"fmt"
	pathpkg "path"
	"path/filepath"
	"strings"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

func New(nixPath string) (db *DB, close func() error, err error) {
	uri := filepath.Join(nixPath, "var", "nix", "db", "db.sqlite")
	db = &DB{
		StorePath: filepath.Join(nixPath, "store"),
	}
	db.pool, err = sqlitex.NewPool(uri, sqlitex.PoolOptions{
		PoolSize: 10,
		Flags:    sqlite.OpenReadWrite,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("db: failed to create pool: %w", err)
	}
	return db, db.pool.Close, nil
}

type DB struct {
	StorePath string
	pool      *sqlitex.Pool
}

func (d *DB) QueryPathFromHashPart(ctx context.Context, hash string) (path string, ok bool, err error) {
	conn, err := d.pool.Take(ctx)
	if err != nil {
		return "", false, fmt.Errorf("db: failed to take connection: %w", err)
	}
	defer d.pool.Put(conn)
	prefix := pathpkg.Join(d.StorePath, hash)
	err = sqlitex.ExecuteTransient(conn, `select path from ValidPaths where path >= ? limit 1;`, &sqlitex.ExecOptions{
		Args: []any{prefix},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			path = stmt.ColumnText(0)
			ok = true
			return nil
		},
	})
	if !strings.HasPrefix(path, prefix) {
		return "", false, nil
	}
	return path, ok, err
}

type PathInfo struct {
	ID               int
	Hash             string
	RegistrationTime int
	Deriver          string
	NarSize          int
	Ultimate         bool
	Refs             []string
	Sigs             []string
	CA               string
}

func (d *DB) QueryPathInfo(ctx context.Context, storePath string) (pathInfo PathInfo, ok bool, err error) {
	conn, err := d.pool.Take(ctx)
	if err != nil {
		return pathInfo, false, fmt.Errorf("db: failed to take connection: %w", err)
	}
	defer d.pool.Put(conn)
	q := `select id, hash, registrationTime, deriver, narSize, ultimate, sigs, ca from ValidPaths where path = ?;`
	err = sqlitex.ExecuteTransient(conn, q, &sqlitex.ExecOptions{
		Args: []any{storePath},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			pathInfo.ID = stmt.ColumnInt(0)
			pathInfo.Hash = stmt.ColumnText(1)
			pathInfo.RegistrationTime = stmt.ColumnInt(2)
			pathInfo.Deriver = d.stripPath(stmt.ColumnText(3))
			pathInfo.NarSize = stmt.ColumnInt(4)
			pathInfo.Ultimate = stmt.ColumnInt(5) == 1
			pathInfo.Sigs = newStringSet(stmt.ColumnText(6), " ")
			pathInfo.CA = stmt.ColumnText(7)
			ok = true
			return nil
		},
	})
	pathInfo.Refs, err = d.QueryReferences(ctx, pathInfo.ID)
	if err != nil {
		return pathInfo, false, fmt.Errorf("db: failed to query references: %w", err)
	}

	return pathInfo, ok, err
}

func (d *DB) stripPath(s string) string {
	prefix := d.StorePath
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return strings.TrimPrefix(s, prefix)
}

func (d *DB) stripPathAll(ss []string) []string {
	result := make([]string, len(ss))
	prefix := d.StorePath
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	for i, s := range ss {
		result[i] = strings.TrimPrefix(s, prefix)
	}
	return result
}

func (d *DB) QueryReferences(ctx context.Context, id int) (references []string, err error) {
	conn, err := d.pool.Take(ctx)
	if err != nil {
		return nil, fmt.Errorf("db: failed to take connection: %w", err)
	}
	defer d.pool.Put(conn)
	q := `select path from Refs join ValidPaths on reference = id where referrer = ?;`
	err = sqlitex.ExecuteTransient(conn, q, &sqlitex.ExecOptions{
		Args: []any{id},
		ResultFunc: func(stmt *sqlite.Stmt) error {
			path := stmt.ColumnText(0)
			// Canonicalize the path.
			path, err = filepath.EvalSymlinks(path)
			if err != nil {
				return fmt.Errorf("db: failed to eval symlink %q: %w", stmt.ColumnText(0), err)
			}
			path, err = filepath.Abs(path)
			if err != nil {
				return fmt.Errorf("db: failed to get abs path %q: %w", stmt.ColumnText(0), err)
			}
			references = append(references, d.stripPath(path))
			return nil
		},
	})
	return references, err
}

func newStringSet(s, separator string) (values []string) {
	set := map[string]struct{}{}
	for _, v := range strings.Split(s, separator) {
		if _, inSet := set[v]; inSet {
			continue
		}
		set[v] = struct{}{}
		values = append(values, v)
	}
	return values
}
