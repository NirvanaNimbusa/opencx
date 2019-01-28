package ocxsql

import (
	"fmt"
	"os"
	"log"
	"io"
	"database/sql"

	// mysql is just the driver, always interact with database/sql api
	_ "github.com/go-sql-driver/mysql"
)

// turn into config options
var (
	defaultUsername = "opencx"
	defaultPassword = "testpass"

	// definitely move this to a config file
	rootPass        = ""
	balanceSchema   = "balances"
	depositSchema   = "deposit"
	assetArray      = []string{"btc", "ltc", "vtc"}
)

// DB contains the sql DB type as well as a logger
type DB struct {
	DBHandler     *sql.DB
	logger        *log.Logger
	balanceSchema string
	depositSchema string
	assetArray    []string
}

// SetupClient sets up the mysql client and driver
func(db *DB) SetupClient() error {
	var err error

	db.balanceSchema = balanceSchema
	db.depositSchema = depositSchema
	// Create users and schemas and assign permissions to opencx
	err = db.RootInitSchemas(rootPass)
	if err != nil {
		return fmt.Errorf("Root could not initialize schemas: \n%s", err)
	}

	// open db handle
	dbHandle, err := sql.Open("mysql", defaultUsername + ":" + defaultPassword + "@/")
	if err != nil {
		return fmt.Errorf("Error opening database: \n%s", err)
	}

	db.DBHandler = dbHandle
	db.assetArray = assetArray

	err = db.DBHandler.Ping()
	if err != nil {
		return fmt.Errorf("Could not ping the database, is it running: \n%s", err)
	}

	// Initialize Balance tables (order tables soon)
	// hacky workaround to get behind the fact I made a dumb abstraction with InitializeTables
	err = db.InitializeTables(balanceSchema, "name TEXT, balance BIGINT")
	if err != nil {
		return fmt.Errorf("Could not initialize balance tables: \n%s", err)
	}

	// Initialize Deposit tables (order tables soon)
	err = db.InitializeTables(depositSchema, "name TEXT, address TEXT")
	if err != nil {
		return fmt.Errorf("Could not initialize deposit tables: \n%s", err)
	}

	return nil
}

// SetLogPath sets the log path for the database, and tells it to also print to stdout. This should be changed in the future so only verbose clients log to stdout
func (db *DB) SetLogPath(logPath string) error {
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(err)
	}

	mw := io.MultiWriter(os.Stdout, logFile)
	db.logger = log.New(mw, "OPENCX DATABASE: ", log.LstdFlags)
	db.LogPrintf("Logger has been set up at %s\n", logPath)
	return nil
}

// These methods can be removed, but these are used frequently so maybe the
// time spent writing these cuts down on the time spent writing logger

// LogPrintf is like printf but you don't have to go db.logger every time
func (db *DB) LogPrintf(format string, v ...interface{}) {
	db.logger.Printf(format, v...)
}

// LogPrintln is like println but you don't have to go db.logger every time
func (db *DB) LogPrintln(v ...interface{}) {
	db.logger.Println(v...)
}

// LogPrint is like print but you don't have to go db.logger every time
func (db *DB) LogPrint(v ...interface{}) {
	db.logger.Print(v...)
}

// LogErrorf is like printf but with error at the beginning
func (db *DB) LogErrorf(format string, v ...interface{}) {
	db.logger.Printf("ERROR: "+format, v...)
}

// InitializeTables initializes all of the tables necessary for the exchange to run. The schema string can be either balanceSchema or depositSchema.
func (db *DB) InitializeTables(schemaName string, schemaSpec string) error {
	var err error

	// Use the balance schema
	_, err = db.DBHandler.Exec("USE " + schemaName + ";")
	if err != nil {
		return fmt.Errorf("Could not use balance schema: \n%s", err)
	}

	for _, assetString := range db.assetArray {

		tableQuery := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s);", assetString, schemaSpec)
		_, err = db.DBHandler.Exec(tableQuery)
		if err != nil {
			return fmt.Errorf("Could not create table %s: \n%s", assetString, err)
		}
	}
	return nil
}

// RootInitSchemas initalizes the schemas, creates users, and grants permissions to those users
func(db *DB) RootInitSchemas(rootPassword string) error {
	var err error

	// Log in to root
	rootHandler, err := sql.Open("mysql", "root:" + rootPassword + "@/")
	if err != nil {
		return fmt.Errorf("Error opening root db: \n%s", err)
	}

	// When the method is done, close the root connection
	defer rootHandler.Close()

	err = rootHandler.Ping()
	if err != nil {
		return fmt.Errorf("Could not ping the database, is it running: \n%s", err)
	}

	createUserQuery := fmt.Sprintf("CREATE OR REPLACE USER '%s'@'localhost' IDENTIFIED BY '%s';", defaultUsername, defaultPassword)
	_, err = rootHandler.Exec(createUserQuery)
	if err != nil {
		return fmt.Errorf("Could not create default user: \n%s", err)
	}

	// check balance schema
	// if balance schema not there make it
	_, err = rootHandler.Exec("CREATE SCHEMA IF NOT EXISTS " + db.balanceSchema + ";")
	if err != nil {
		return fmt.Errorf("Could not create balance schema: \n%s", err)
	}

	// grant permissions to default user
	balancePermsQuery := fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, CREATE, DELETE, DROP ON %s.* TO '%s'@'localhost';", db.balanceSchema, defaultUsername)
	_, err = rootHandler.Exec(balancePermsQuery)
	if err != nil {
		return fmt.Errorf("Could not grant permissions to %s while creating balance table: \n%s", defaultUsername, err)
	}

	// check deposit schema
	// if deposit schema not there make it
	_, err = rootHandler.Exec("CREATE SCHEMA IF NOT EXISTS " + db.depositSchema + ";")
	if err != nil {
		return fmt.Errorf("Could not create deposit schema: \n%s", err)
	}

	// grant permissions to default user
	depositPermsQuery := fmt.Sprintf("GRANT SELECT, INSERT, UPDATE, CREATE, DELETE, DROP ON %s.* TO '%s'@'localhost';", db.depositSchema, defaultUsername)
	_, err = rootHandler.Exec(depositPermsQuery)
	if err != nil {
		return fmt.Errorf("Could not grant permissions to %s while creating deposit table: \n%s", defaultUsername, err)
	}

	return nil
}