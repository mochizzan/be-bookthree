package controllers

import (
	"be/config"
	"be/models"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// 1. CREATE TRANSACTION (Checkout dari React)
func CheckoutHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var txData models.Transaction
	if err := json.NewDecoder(r.Body).Decode(&txData); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// --- 1. GENERATE ORDER CODE DI BACKEND ---
	// Format: B3-YYYYMMDD-XXXX (4 digit random angka/huruf)

	// Seed random number generator (agar acak setiap waktu)
	rand.Seed(time.Now().UnixNano())

	// Buat komponen tanggal: 20250130
	dateStr := time.Now().Format("20060102")

	// Buat komponen random: Angka 1000-9999
	randomNum := rand.Intn(9000) + 1000

	// Gabungkan menjadi Order Code
	generatedOrderCode := fmt.Sprintf("B3-%s-%d", dateStr, randomNum)

	// --- 2. MULAI DATABASE TRANSACTION ---
	tx, err := config.DB.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Step 1: Insert Header Transaksi
	// PERHATIKAN: Kita menggunakan 'generatedOrderCode' di sini, BUKAN 'txData.OrderCode'
	res, err := tx.Exec("INSERT INTO transactions (order_code, customer_name, customer_email, customer_phone, customer_address, payment_method, total_amount, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		generatedOrderCode,
		txData.CustomerName,
		txData.CustomerEmail, // <--- Masukkan Email
		txData.CustomerPhone,
		txData.Address,
		txData.PaymentMethod,
		txData.TotalAmount,
		100,
		time.Now())

	if err != nil {
		tx.Rollback()
		http.Error(w, "Gagal simpan transaksi: "+err.Error(), http.StatusInternalServerError)
		return
	}

	txID, _ := res.LastInsertId()

	// Step 2: Insert Detail Buku
	for _, item := range txData.Details {
		_, err = tx.Exec("INSERT INTO transaction_details (transaction_id, book_id, quantity, price_at_purchase) VALUES (?, ?, ?, ?)",
			txID, item.BookID, item.Quantity, item.Price)

		if err != nil {
			tx.Rollback()
			http.Error(w, "Gagal simpan detail: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Update Stok
		_, err = tx.Exec("UPDATE books SET stock = stock - ? WHERE id = ?", item.Quantity, item.BookID)
		if err != nil {
			tx.Rollback()
			http.Error(w, "Gagal update stok", http.StatusInternalServerError)
			return
		}
	}

	// Step 3: Commit
	err = tx.Commit()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)

	// --- 3. KIRIM KODE YANG DIGENERATE KE FRONTEND ---
	response := map[string]interface{}{
		"message":        "Transaksi berhasil disimpan",
		"order_code":     generatedOrderCode, // <--- Ini yang penting!
		"transaction_id": txID,
	}

	json.NewEncoder(w).Encode(response)
}

// 2. GET ALL TRANSACTIONS (Untuk Admin Dashboard)
func TransactionListHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)

	// 1. Query Data Transaksi Utama
	// Tambahkan customer_email di SELECT
	rows, err := config.DB.Query(`
		SELECT id, order_code, customer_name, customer_email, customer_phone, customer_address, 
		       total_amount, status, payment_method, created_at 
		FROM transactions 
		ORDER BY created_at DESC
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var transactions []models.Transaction

	for rows.Next() {
		var t models.Transaction

		// Gunakan sql.NullString untuk antisipasi jika data kosong
		var email, phone, address sql.NullString

		if err := rows.Scan(
			&t.ID,
			&t.OrderCode,
			&t.CustomerName,
			&email, // <--- Scan Email
			&phone,
			&address,
			&t.TotalAmount,
			&t.Status,
			&t.PaymentMethod,
			&t.Date,
		); err != nil {
			log.Println("Scan error:", err)
			continue
		}

		if email.Valid {
			t.CustomerEmail = email.String
		} // <--- Assign Email
		if phone.Valid {
			t.CustomerPhone = phone.String
		}
		if address.Valid {
			t.Address = address.String
		}

		// --- 2. LOGIC AMBIL DETAIL BUKU (Nested Query) ---
		detailRows, err := config.DB.Query(`
			SELECT td.id, td.book_id, td.quantity, td.price_at_purchase, b.title, b.image_url
			FROM transaction_details td
			JOIN books b ON td.book_id = b.id
			WHERE td.transaction_id = ?
		`, t.ID)

		if err != nil {
			log.Println("Detail query error:", err) // Cek log ini jika masih error
		} else {
			var details []models.TransactionDetail
			for detailRows.Next() {
				var d models.TransactionDetail
				var bookTitle, bookImage string

				// Scan detail
				err := detailRows.Scan(&d.ID, &d.BookID, &d.Quantity, &d.Price, &bookTitle, &bookImage)
				if err != nil {
					log.Println("Scan detail error:", err)
					continue
				}

				d.Book.Title = bookTitle
				d.Book.Image = bookImage

				details = append(details, d)
			}
			detailRows.Close()
			t.Details = details
		}

		transactions = append(transactions, t)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(transactions)
}

// 3. UPDATE STATUS (Untuk Admin: Proses/Kirim/Selesai/Batal)
func TransactionStatusHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	// Ambil ID dari URL (/api/transactions/{id}/status)
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	idStr := parts[3] // index ke-3 adalah ID
	id, _ := strconv.Atoi(idStr)

	if r.Method != "PUT" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Struct kecil untuk menampung JSON input { "status": 101 }
	type StatusReq struct {
		Status int `json:"status"`
	}
	var req StatusReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err := config.DB.Exec("UPDATE transactions SET status = ? WHERE id = ?", req.Status, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Status updated successfully"})
}

func GetTransactionByCodeHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" {
		return
	}

	// Ambil code dari URL query param
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Order Code is required", http.StatusBadRequest)
		return
	}

	// Variable penampung
	var t models.Transaction

	// 1. Query Header Transaksi
	// HAPUS 'dateStr', scan langsung ke &t.Date
	row := config.DB.QueryRow(`
		SELECT id, order_code, customer_name, total_amount, status, payment_method, created_at 
		FROM transactions 
		WHERE order_code = ?`, code)

	err := row.Scan(
		&t.ID,
		&t.OrderCode,
		&t.CustomerName,
		&t.TotalAmount,
		&t.Status,
		&t.PaymentMethod,
		&t.Date, // <--- Langsung scan ke time.Time
	)

	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Pesanan tidak ditemukan", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 2. Query Detail Barang (Agar user tahu beli apa)
	// Kita gunakan 'price_at_purchase' karena nama kolomnya itu
	rows, err := config.DB.Query(`
		SELECT td.quantity, td.price_at_purchase, b.title, b.image_url 
		FROM transaction_details td 
		JOIN books b ON td.book_id = b.id 
		WHERE td.transaction_id = ?`, t.ID)

	if err == nil {
		defer rows.Close()

		var details []models.TransactionDetail
		for rows.Next() {
			var d models.TransactionDetail

			// Scan data detail
			err := rows.Scan(&d.Quantity, &d.Price, &d.Book.Title, &d.Book.Image)
			if err != nil {
				continue
			}

			details = append(details, d)
		}
		t.Details = details
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}
