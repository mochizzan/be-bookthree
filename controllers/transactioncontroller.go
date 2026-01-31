package controllers

import (
	"be/config"
	"be/models"
	"database/sql"
	"encoding/json"
	"fmt"
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
	res, err := tx.Exec("INSERT INTO transactions (order_code, customer_name, customer_phone, customer_address, payment_method, total_amount, status, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		generatedOrderCode, txData.CustomerName, txData.CustomerPhone, txData.Address, txData.PaymentMethod, txData.TotalAmount, 100, time.Now())

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
	if r.Method == "OPTIONS" {
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rows, err := config.DB.Query("SELECT id, order_code, customer_name, total_amount, status, payment_method, created_at FROM transactions ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var transactions []models.Transaction
	for rows.Next() {
		var t models.Transaction
		var dateStr string
		// Scan (Perhatikan urutan kolom di query)
		if err := rows.Scan(&t.ID, &t.OrderCode, &t.CustomerName, &t.TotalAmount, &t.Status, &t.PaymentMethod, &dateStr); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		t.Date = dateStr
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

	// Ambil code dari URL query param: /api/check-order?code=B3-XXXX
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Order Code is required", http.StatusBadRequest)
		return
	}

	// Query Header Transaksi
	var t models.Transaction
	var dateStr string

	row := config.DB.QueryRow("SELECT id, order_code, customer_name, total_amount, status, payment_method, created_at FROM transactions WHERE order_code = ?", code)

	err := row.Scan(&t.ID, &t.OrderCode, &t.CustomerName, &t.TotalAmount, &t.Status, &t.PaymentMethod, &dateStr)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Pesanan tidak ditemukan", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	t.Date = dateStr

	// Query Detail Barang (Optional: Biar user tahu dia beli apa)
	rows, err := config.DB.Query("SELECT b.title, td.quantity FROM transaction_details td JOIN books b ON td.book_id = b.id WHERE td.transaction_id = ?", t.ID)
	if err == nil {
		// Kita reuse struct TransactionDetail tapi kali ini kita inject Judul Buku manual ke struct (atau bikin struct baru, tapi biar cepat kita pakai map dulu untuk response ini)
		// Agar simple, kita kirim header saja dulu, detailnya nanti bisa dikembangkan.
		defer rows.Close()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}
