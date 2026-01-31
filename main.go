package main

import (
	"be/config"
	"be/controllers"
	"fmt"
	"net/http"
)

func main() {
	config.ConnectDB()

	// --- ROUTING API ---
	http.HandleFunc("/api/books", controllers.BooksHandler)
	http.HandleFunc("/api/books/", controllers.BookDetailHandler)
	http.HandleFunc("/api/login", controllers.LoginHandler)
	http.HandleFunc("/api/checkout", controllers.CheckoutHandler)
	http.HandleFunc("/api/transactions", controllers.TransactionListHandler)
	http.HandleFunc("/api/transactions/", controllers.TransactionStatusHandler)
	http.HandleFunc("/api/check-order", controllers.GetTransactionByCodeHandler)

	http.HandleFunc("/api/upload", controllers.UploadHandler)

	fs := http.FileServer(http.Dir("./uploads"))
	http.Handle("/uploads/", http.StripPrefix("/uploads/", fs))

	port := ":8082"
	fmt.Println("Server running on port", port)
	err := http.ListenAndServe(port, nil)
	if err != nil {
		panic(err)
	}
}
