package sqlite

import (
	"database/sql"
	"log"
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
			log.Println("Error al crear directorio de DB.", err)
			return err
		}
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		_, err := os.Create(dbPath)
		if err != nil {
			log.Println("Error al crear DB.", err)
			return err
		}

		database, err := ConnectDB(dbPath)
		if err != nil {
			log.Println("Error al conectarse a DB.", err)
			return err
		}
		db = database

		if _, err := ExecuteNonQuery(QueryCreateTables()); err != nil {
			log.Println("Error creando tablas en DB.", err)
			return err
		}
	} else {
		database, err := ConnectDB(dbPath)
		if err != nil {
			return err
		}
		db = database
	}
	log.Println("Conexión exitosa a DB.")
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
	);
	
	-- DATA DUMMY PARA PRUEBAS
	INSERT INTO DEVICE (ID_DEVICE, FIELD, VALUE) VALUES ('SBC001', 'location', 'warehouse');

	INSERT INTO DEVICE_READING_SETTINGS (ID_DEVICE, PARAMETER, PERIOD, ACTIVE) VALUES ('SBC001', 'temperatura', 10, 1);
	INSERT INTO DEVICE_READING_SETTINGS (ID_DEVICE, PARAMETER, PERIOD, ACTIVE) VALUES ('SBC001', 'ram', 30, 1);
	INSERT INTO DEVICE_READING_SETTINGS (ID_DEVICE, PARAMETER, PERIOD, ACTIVE) VALUES ('SBC001', 'disk', 60, 0);

	INSERT INTO DEVICE_UPDATES (ID_DEVICE, UPDATE_DATETIME_UTC) VALUES ('SBC001', '2024-05-22T12:00:00Z');
	`
}

func GetDeviceReadingSettings() ([]DeviceReadingSetting, error) {
	query := "SELECT ID_DEVICE, PARAMETER, PERIOD, ACTIVE FROM DEVICE_READING_SETTINGS"
	rows, err := ExecuteQuery(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := []DeviceReadingSetting{}
	for rows.Next() {
		var setting DeviceReadingSetting
		if err := rows.Scan(&setting.IDDevice, &setting.Parameter, &setting.Period, &setting.Active); err != nil {
			return nil, err
		}
		settings = append(settings, setting)
	}
	return settings, nil
}

func GetDevices() ([]Device, error) {
	query := "SELECT ID_DEVICE, FIELD, VALUE FROM DEVICE"
	rows, err := ExecuteQuery(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	devices := []Device{}
	for rows.Next() {
		var device Device
		if err := rows.Scan(&device.IDDevice, &device.Field, &device.Value); err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, nil
}

func GetDeviceUpdates() ([]DeviceUpdate, error) {
	query := "SELECT ID_DEVICE, UPDATE_DATETIME_UTC FROM DEVICE_UPDATES"
	rows, err := ExecuteQuery(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	updates := []DeviceUpdate{}
	for rows.Next() {
		var update DeviceUpdate
		if err := rows.Scan(&update.IDDevice, &update.UpdateDatetimeUTC); err != nil {
			return nil, err
		}
		updates = append(updates, update)
	}
	return updates, nil
}
