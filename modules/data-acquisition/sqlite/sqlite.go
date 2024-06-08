package sqlite

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"time"

	"ceiot-tf-sbc/modules/data-acquisition/models"

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

		if _, err := db.Exec(QueryCreateTables()); err != nil {
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
	return `CREATE TABLE IF NOT EXISTS MAIN_DEVICE_INFORMATION (
		ID_DEVICE TEXT NOT NULL,
		HOSTNAME TEXT,
		PROCESSOR TEXT,
		RAM TEXT,
		HOSTID TEXT,
		OS TEXT,
		KERNEL TEXT,
		PRIMARY KEY (ID_DEVICE)
	);

	CREATE TABLE IF NOT EXISTS DEVICE_READING_SETTINGS (
		ID_DEVICE TEXT,
		PARAMETER TEXT,
		PERIOD INTEGER,
		ACTIVE BOOLEAN,
		PRIMARY KEY (ID_DEVICE, PARAMETER)
	);
	
	CREATE TABLE IF NOT EXISTS DEVICE_UPDATES (
		ID_DEVICE TEXT,
		HASH_UPDATE TEXT,
		TYPE TEXT,
		UPDATE_DATETIME_UTC TEXT,
		PRIMARY KEY (ID_DEVICE, HASH_UPDATE)
	);
	`
}

func GetDeviceReadingSettings() ([]models.DeviceReadingSetting, error) {
	query := "SELECT ID_DEVICE, PARAMETER, PERIOD, ACTIVE FROM DEVICE_READING_SETTINGS"
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := []models.DeviceReadingSetting{}
	for rows.Next() {
		var setting models.DeviceReadingSetting
		if err := rows.Scan(&setting.DeviceID, &setting.Parameter, &setting.Period, &setting.Active); err != nil {
			return nil, err
		}
		settings = append(settings, setting)
	}
	return settings, nil
}

func GetMainDeviceInformation() (map[string]interface{}, error) {
	query := "SELECT HOSTNAME, PROCESSOR, RAM, HOSTID, OS, KERNEL FROM MAIN_DEVICE_INFORMATION"
	row := db.QueryRow(query)

	var hostname, processor, ram, hostID, os, kernel string
	err := row.Scan(
		&hostname,
		&processor,
		&ram,
		&hostID,
		&os,
		&kernel,
	)
	if err != nil {
		return nil, err
	}

	mainDeviceInformation := map[string]interface{}{
		"hostname":  hostname,
		"processor": processor,
		"ram":       ram,
		"hostID":    hostID,
		"os":        os,
		"kernel":    kernel,
	}

	return mainDeviceInformation, nil
}

func GetDeviceUpdates(updateType string, hashUpdate string) ([]models.DeviceUpdate, error) {
	query := "SELECT ID_DEVICE, HASH_UPDATE, TYPE, UPDATE_DATETIME_UTC FROM DEVICE_UPDATES WHERE TYPE = ? AND HASH_UPDATE = ?"
	rows, err := db.Query(query, updateType, hashUpdate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	updates := []models.DeviceUpdate{}
	for rows.Next() {
		var update models.DeviceUpdate
		if err := rows.Scan(&update.DeviceID, &update.HashUpdate, &update.Type, &update.UpdateDatetimeUTC); err != nil {
			return nil, err
		}
		updates = append(updates, update)
	}
	return updates, nil
}

func UpdateSettings(hashUpdate string, updateType string, newSettings []models.DeviceReadingSetting) (time.Time, error) {
	tx, err := db.Begin()
	if err != nil {
		return time.Time{}, err
	}

	for _, newSetting := range newSettings {
		var exists bool
		err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM DEVICE_READING_SETTINGS WHERE PARAMETER = ? AND ID_DEVICE = ?)", newSetting.Parameter, newSetting.DeviceID).Scan(&exists)
		if err != nil {
			tx.Rollback()
			return time.Time{}, err
		}

		if exists {
			_, err = tx.Exec("UPDATE DEVICE_READING_SETTINGS SET PERIOD = ?, ACTIVE = ? WHERE PARAMETER = ? AND ID_DEVICE = ?", newSetting.Period, newSetting.Active, newSetting.Parameter, newSetting.DeviceID)
		} else {
			_, err = tx.Exec("INSERT INTO DEVICE_READING_SETTINGS (ID_DEVICE, PARAMETER, PERIOD, ACTIVE) VALUES (?, ?, ?, ?)", newSetting.DeviceID, newSetting.Parameter, newSetting.Period, newSetting.Active)
		}

		if err != nil {
			tx.Rollback()
			return time.Time{}, err
		}
	}

	utcTime := time.Now().UTC()
	formattedTime := utcTime.Format(time.RFC3339)
	_, err = tx.Exec("INSERT INTO DEVICE_UPDATES (ID_DEVICE, HASH_UPDATE, TYPE, UPDATE_DATETIME_UTC) VALUES (?, ?, ?, ?)", newSettings[0].DeviceID, hashUpdate, updateType, formattedTime)
	if err != nil {
		tx.Rollback()
		return time.Time{}, err
	}

	err = tx.Commit()
	if err != nil {
		return time.Time{}, err
	}

	return utcTime, nil
}

func InsertMainDeviceInformation(DeviceID string, mainDeviceInformation map[string]interface{}) error {
	log.Println(mainDeviceInformation)
	for key := range mainDeviceInformation {
		log.Println("Key:", key)
	}
	_, err := db.Exec(`
		INSERT INTO MAIN_DEVICE_INFORMATION (
			ID_DEVICE, 
			HOSTNAME, 
			PROCESSOR, 
			RAM, 
			HOSTID, 
			OS, 
			KERNEL
		) VALUES (?, ?, ?, ?, ?, ?, ?)
	`,
		DeviceID,
		mainDeviceInformation["hostname"],
		mainDeviceInformation["processor"],
		mainDeviceInformation["ram"],
		mainDeviceInformation["hostID"],
		mainDeviceInformation["os"],
		mainDeviceInformation["kernel"],
	)
	if err != nil {
		return err
	}

	return nil
}
