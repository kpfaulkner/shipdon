package main

import (
	"database/sql"
	"database/sql/driver"
	"git.sr.ht/~gioverse/skel/stream"
	"modernc.org/sqlite"

	_ "modernc.org/sqlite"
)

type sqliteDriver struct {
	*sqlite.Driver
}

func (d sqliteDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.Driver.Open(name)
	if err != nil {
		return conn, err
	}
	c := conn.(interface {
		Exec(stmt string, args []driver.Value) (driver.Result, error)
	})
	if _, err := c.Exec("PRAGMA foreign_keys = on;", nil); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func init() {
	sql.Register("sqlite3", sqliteDriver{Driver: &sqlite.Driver{}})
	//sql.Register(sqlite3, &sqlite3.SQLiteDriver{
	//	Extensions: []string{},
	//	ConnectHook: func(sqliteConn *sqlite3.SQLiteConn) error {
	//		sqliteConn.RegisterUpdateHook(b.tracker.connectHook)
	//		return nil
	//	},
	//})
}

// Task is our in-memory representation of a todo task.
type Task struct {
	ID      int64
	Name    string
	Details string
	Done    bool
}

// Scan extracts a task from a database row containing
// id, name, details, status in that order.
func (i *Task) Scan(rows *sql.Rows) error {
	if err := rows.Scan(&i.ID, &i.Name, &i.Details, &i.Done); err != nil {
		return err
	}
	return nil
}

type TaskMutation = *stream.Mutation[stream.Result[Task]]
type TaskMutationMap = map[int64]TaskMutation

// dbOp represents a kind of change within the database.
type dbOp uint8
