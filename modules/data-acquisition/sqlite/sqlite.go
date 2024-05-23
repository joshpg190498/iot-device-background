package sqlite

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func ConnectDB(dbPath string) (*sql.DB, error) {
	database, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}
	return database, nil
}

func ExecuteNonQuery(query string) (bool, error) {
	_, err := db.Exec(query)
	if err != nil {
		return false, err
	}
	return true, nil
}

func ExecuteQuery(query string, args ...interface{}) (*sql.Rows, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func CloseDB() error {
	return db.Close()
}

func InitDB(dbPath string) error {
	dir := filepath.Dir(dbPath)

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Println("Error al crear directorio de DB.")
			return err
		}
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		_, err := os.Create(dbPath)
		if err != nil {
			fmt.Println("Error al crear DB.")
			return err
		}

		database, err := ConnectDB(dbPath)
		if err != nil {
			fmt.Println("Error al conectarse a DB.")
			return err
		}
		db = database

		if _, err := ExecuteNonQuery(QueryCreateTables()); err != nil {
			fmt.Println("Error creando tablas en DB.")
			return err
		}
	} else {
		database, err := ConnectDB(dbPath)
		if err != nil {
			return err
		}
		db = database
	}
	fmt.Println("Conexión exitosa a DB.")
	return nil
}

func QueryCreateTables() string {
	return `CREATE TABLE IF NOT EXISTS DEVICE (
		ID_DEVICE TEXT PRIMARY KEY,
		FIELD TEXT,
		VALUE TEXT
	);

	CREATE TABLE IF NOT EXISTS DEVICE_READING_SETTINGS (
		ID_DEVICE TEXT,
		PARAMETER TEXT,
		PERIOD INTEGER,
		ACTIVE BOOLEAN,
		PRIMARY KEY (ID_DEVICE, PARAMETER),
		FOREIGN KEY (ID_DEVICE) REFERENCES DEVICE(ID_DEVICE)
	);
	
	CREATE TABLE IF NOT EXISTS DEVICE_UPDATES (
		ID_DEVICE TEXT,
		UPDATE_DATETIME_UTC TEXT
	)
	`
}
