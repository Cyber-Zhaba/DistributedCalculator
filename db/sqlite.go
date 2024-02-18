package db

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"strconv"
)

type DB struct {
	*sql.DB
}

func (db *DB) Init() error {
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS Equations (ID INTEGER PRIMARY KEY AUTOINCREMENT, text TEXT, status TEXT, result REAL)")
	if err != nil {
		return err
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS Operations (type TEXT PRIMARY KEY, duration INTEGER)")
	if err != nil {
		return err
	}
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS Computers (ID INTEGER PRIMARY KEY AUTOINCREMENT, EquationID INTEGER, FOREIGN KEY (EquationID) REFERENCES Equations(ID))")
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT OR IGNORE INTO Operations (type, duration) VALUES ('+', 1)")
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT OR IGNORE INTO Operations (type, duration) VALUES ('-', 1)")
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT OR IGNORE INTO Operations (type, duration) VALUES ('*', 1)")
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT OR IGNORE INTO Operations (type, duration) VALUES ('/', 1)")
	if err != nil {
		return err
	}
	return nil
}

// Connect to the SQLite database.
func Connect(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

// GetAllValues retrieves all values from the specified table.
func (db *DB) GetAllValues(tableName string) ([]map[string]interface{}, error) {
	rows, err := db.Query("SELECT * FROM " + tableName)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			return
		}
	}(rows)

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtr := make([]interface{}, len(columns))
		for i := range columns {
			valuePtr[i] = &values[i]
		}

		if err := rows.Scan(valuePtr...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, column := range columns {
			val := values[i]
			if val != nil {
				row[column] = val
			} else {
				row[column] = nil
			}
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

// AddEquation adds a new row with the given text. The id is auto-incremented.
func (db *DB) AddEquation(id int, text string, tableName string) (int, error) {
	// Prepare the SQL statement
	if id == 0 {
		stmt, err := db.Prepare("INSERT INTO " + tableName + " (text, status, result) VALUES (?, ?, ?)")
		if err != nil {
			return 0, err
		}
		defer func(stmt *sql.Stmt) {
			err = stmt.Close()
			if err != nil {
				return
			}
		}(stmt)

		// Execute the SQL statement
		_, err = stmt.Exec(text, "In queue", 0)
		if err != nil {
			return 0, err
		}
		lastId, _ := db.Query("SELECT ID FROM Equations ORDER BY ID DESC LIMIT 1")
		defer func(lastId *sql.Rows) {
			err = lastId.Close()
			if err != nil {
				return
			}

		}(lastId)
		if lastId.Next() {
			err = lastId.Scan(&id)
			if err != nil {
				return 0, err
			}
			return id, nil
		}
		return 0, nil
	} else {
		stmt, err := db.Prepare("INSERT OR IGNORE INTO " + tableName + " (ID, text, status, result) VALUES (?, ?, ?, ?)")
		if err != nil {
			return 0, err
		}
		defer func(stmt *sql.Stmt) {
			err = stmt.Close()
			if err != nil {
				return
			}
		}(stmt)

		// Execute the SQL statement
		_, err = stmt.Exec(id, text, "in queue", 0)
		if err != nil {
			return 0, err
		}
		return id, nil
	}
}

func (db *DB) UpdateOperations(operationType []string, duration []string) error {
	// Prepare the SQL statement
	for i, opType := range operationType {
		stmt, err := db.Prepare("UPDATE Operations SET duration = ? WHERE type = ?")
		if err != nil {
			return err
		}
		// Execute the SQL statement
		durationInt := 0
		durationInt, err = strconv.Atoi(duration[i])
		if err != nil {
			err = stmt.Close()
			if err != nil {
				return err
			}
			continue
		}
		_, err = stmt.Exec(durationInt, opType)
		err = stmt.Close()
		if err != nil {
			return err
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (db *DB) GetEmptyComputer() (int, error) {
	rows, err := db.Query("SELECT ID FROM Computers WHERE EquationID IS NULL LIMIT 1")
	if err != nil {
		return 0, err
	}
	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			return
		}
	}(rows)
	var id int
	if rows.Next() {
		err = rows.Scan(&id)
		if err != nil {
			return 0, err
		}
	}
	return id, nil
}

func (db *DB) UpdateComputer(id int, equationID int) error {
	// if equationID is 0, set null
	stmt, err := db.Prepare("UPDATE Computers SET EquationID = ? WHERE ID = ?")
	if equationID == 0 {
		stmt, err = db.Prepare("UPDATE Computers SET EquationID = NULL WHERE ID = ?")
	}
	if err != nil {
		return err
	}
	defer func(stmt *sql.Stmt) {
		err = stmt.Close()
		if err != nil {
			return
		}
	}(stmt)
	if equationID == 0 {
		_, err = stmt.Exec(id)
	} else {
		_, err = stmt.Exec(equationID, id)
	}
	if err != nil {
		return err
	}
	return nil
}

func (db *DB) UpdateEquation(id int, status string, result float64) error {
	stmt, err := db.Prepare("UPDATE Equations SET status = ?, result = ? WHERE ID = ?")
	if err != nil {
		return err
	}
	defer func(stmt *sql.Stmt) {
		err = stmt.Close()
		if err != nil {
			return
		}
	}(stmt)
	_, err = stmt.Exec(status, result, id)
	if err != nil {
		return err
	}
	return nil
}

func (db *DB) GetOperationTime(operation string) (int, error) {
	rows, err := db.Query("SELECT duration FROM Operations WHERE type = ?", operation)
	if err != nil {
		return 0, err
	}
	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			return
		}
	}(rows)

	var duration int
	if rows.Next() {
		err = rows.Scan(&duration)
		if err != nil {
			return 0, err
		}
	}
	return duration, nil
}

func (db *DB) GetEquationText(id int) string {
	rows, err := db.Query("SELECT text FROM Equations WHERE ID = ?", id)
	if err != nil {
		return ""
	}
	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			return
		}
	}(rows)
	if rows.Next() {
		var equation string
		err = rows.Scan(&equation)
		if err != nil {
			return ""
		}
		return equation
	}
	return ""
}

func (db *DB) GetEquationInfo(id int) (string, string, float64) {
	rows, err := db.Query("SELECT * FROM Equations WHERE ID = ?", id)
	if err != nil {
		return "", "", 0
	}
	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			return
		}
	}(rows)
	if rows.Next() {
		var text string
		var status string
		var result float64
		err = rows.Scan(&id, &text, &status, &result)
		if err != nil {
			return "", "", 0
		}
		return text, status, result
	}
	return "", "", 0
}

func (db *DB) AddComputer() error {
	// Prepare the SQL statement
	stmt, err := db.Prepare("INSERT INTO Computers (EquationID) Values (NULL)")
	if err != nil {
		return err
	}
	defer func(stmt *sql.Stmt) {
		err = stmt.Close()
		if err != nil {
			return
		}
	}(stmt)
	stmt.Exec()
	return nil
}
