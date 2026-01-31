package controllers

import (
	"be/config"
	"be/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Helper untuk mengatur Header CORS (Agar React bisa akses)
func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

// 1. GET ALL & CREATE (URL: /api/books)
func BooksHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	switch r.Method {
	case "GET":
		getBooks(w, r)
	case "POST":
		createBook(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// 2. GET DETAIL, UPDATE, DELETE (URL: /api/books/{id})
func BookDetailHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	// Ambil ID dari URL
	idStr := strings.TrimPrefix(r.URL.Path, "/api/books/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "GET":
		getBook(w, id)
	case "PUT":
		updateBook(w, r, id)
	case "DELETE":
		deleteBook(w, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// --- LOGIC IMPLEMENTATION ---

func getBooks(w http.ResponseWriter, r *http.Request) {
	rows, err := config.DB.Query("SELECT id, title, author, price, category, stock, image_url, description FROM books ORDER BY id DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var books []models.Book
	for rows.Next() {
		var book models.Book
		// Scan urutannya harus sama dengan Query SELECT di atas
		if err := rows.Scan(&book.ID, &book.Title, &book.Author, &book.Price, &book.Category, &book.Stock, &book.ImageURL, &book.Description); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		books = append(books, book)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(books)
}

func getBook(w http.ResponseWriter, id int) {
	var book models.Book
	row := config.DB.QueryRow("SELECT id, title, author, price, category, stock, image_url, description FROM books WHERE id = ?", id)

	err := row.Scan(&book.ID, &book.Title, &book.Author, &book.Price, &book.Category, &book.Stock, &book.ImageURL, &book.Description)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Book not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(book)
}

func createBook(w http.ResponseWriter, r *http.Request) {
	var book models.Book
	// Decode JSON dari body request
	if err := json.NewDecoder(r.Body).Decode(&book); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Default Image jika kosong
	if book.ImageURL == "" {
		book.ImageURL = "https://placehold.co/300x450?text=No+Image"
	}

	result, err := config.DB.Exec("INSERT INTO books (title, author, price, category, stock, image_url, description) VALUES (?, ?, ?, ?, ?, ?, ?)",
		book.Title, book.Author, book.Price, book.Category, book.Stock, book.ImageURL, book.Description)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id, _ := result.LastInsertId()
	book.ID = int(id)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(book)
}

func deleteImage(imageURL string) {
	// 1. Cek apakah ini gambar placeholder / dummy? Jika ya, jangan dihapus.
	if strings.Contains(imageURL, "placehold.co") {
		return
	}

	// 2. Parsing URL untuk dapat nama file lokal
	// URL: http://localhost:8080/uploads/1723123-buku.jpg
	// Kita butuh: uploads/1723123-buku.jpg

	// Cari kata "/uploads/"
	parts := strings.Split(imageURL, "/uploads/")
	if len(parts) < 2 {
		return // Format URL tidak dikenali
	}

	filename := parts[1] // Ambil bagian setelah /uploads/

	// Gabungkan dengan folder lokal
	localPath := filepath.Join("uploads", filename)

	// 3. Hapus file
	err := os.Remove(localPath)
	if err != nil {
		fmt.Println("Gagal menghapus file lama:", err)
		// Kita hanya print error, jangan stop proses update DB
	} else {
		fmt.Println("File lama berhasil dihapus:", localPath)
	}
}

// --- UPDATE FUNGSI updateBook ---
func updateBook(w http.ResponseWriter, r *http.Request, id int) {
	var book models.Book
	if err := json.NewDecoder(r.Body).Decode(&book); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 1. AMBIL DATA LAMA DARI DATABASE (Sebelum Update)
	var oldImageURL string
	row := config.DB.QueryRow("SELECT image_url FROM books WHERE id = ?", id)
	err := row.Scan(&oldImageURL)
	if err != nil {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	// 2. LOGIC HAPUS GAMBAR
	// Jika URL yang dikirim beda dengan URL di database,
	// dan URL di database tidak kosong
	if book.ImageURL != "" && book.ImageURL != oldImageURL {
		deleteImage(oldImageURL) // <--- HAPUS FILE LAMA
	}

	// 3. UPDATE DATABASE (Seperti biasa)
	_, err = config.DB.Exec("UPDATE books SET title=?, author=?, price=?, category=?, stock=?, description=?, image_url=? WHERE id=?",
		book.Title, book.Author, book.Price, book.Category, book.Stock, book.Description, book.ImageURL, id)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	book.ID = id
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Book updated successfully"})
}

func deleteBook(w http.ResponseWriter, id int) {
	// 1. Ambil URL Gambar sebelum dihapus
	var oldImageURL string
	row := config.DB.QueryRow("SELECT image_url FROM books WHERE id = ?", id)
	err := row.Scan(&oldImageURL)

	// Jika buku tidak ada, return error
	if err != nil {
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	// 2. Hapus Data dari DB
	_, err = config.DB.Exec("DELETE FROM books WHERE id=?", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Hapus File Fisik
	deleteImage(oldImageURL)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Book deleted successfully"})
}
