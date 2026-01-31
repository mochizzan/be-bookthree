package controllers

import (
	"be/config"
	"be/models"
	"database/sql"
	"encoding/json"
	"net/http"
)

func LoginHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method == "OPTIONS" { return }

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var user models.User
	// Query cek user (Password masih plain text untuk pembelajaran)
	// Di production WAJIB pakai hashing (bcrypt)
	row := config.DB.QueryRow("SELECT id, username, role FROM users WHERE username=? AND password=?", req.Username, req.Password)
	
	err := row.Scan(&user.ID, &user.Username, &user.Role)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Username atau Password salah", http.StatusUnauthorized)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Login Sukses
	resp := models.LoginResponse{
		Status:  true,
		Message: "Login Berhasil",
		Token:   "dummy-token-secret-123", // Di real app gunakan JWT Library
		Role:    user.Role,
		Username: user.Username,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}