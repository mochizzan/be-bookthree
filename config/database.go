package config

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

func ConnectDB() {
	var err error
	// Format: username:password@tcp(host:port)/nama_database
	// Sesuaikan dengan user/pass mysql kamu (biasanya root dan kosong)
	connectionString := "root:@Miproduction04@tcp(127.0.0.1:3306)/bookthree_db"

	DB, err = sql.Open("mysql", connectionString)
	if err != nil {
		panic(err)
	}

	// Tes koneksi
	err = DB.Ping()
	if err != nil {
		panic(err)
	}

	fmt.Println("Database Connected Successfully!")
}
