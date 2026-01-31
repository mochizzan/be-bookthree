package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func UploadHandler(w http.ResponseWriter, r *http.Request) {
	// Enable CORS
	enableCors(&w)
	if r.Method == "OPTIONS" { return }

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Parse Multipart Form (Max 10 MB)
	r.ParseMultipartForm(10 << 20)

	// 2. Ambil file dari form key "image"
	file, handler, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 3. Buat folder 'uploads' jika belum ada
	if _, err := os.Stat("uploads"); os.IsNotExist(err) {
		os.Mkdir("uploads", 0755)
	}

	// 4. Buat nama file unik (Timestamp + Nama Asli)
	// Contoh: 178234234-harrypotter.jpg
	filename := fmt.Sprintf("%d-%s", time.Now().Unix(), handler.Filename)
	filePath := filepath.Join("uploads", filename)

	// 5. Simpan File ke Server
	dst, err := os.Create(filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 6. Return URL Lengkap Gambar
	// URL ini akan disimpan di tabel books
	fileURL := fmt.Sprintf("http://localhost:8080/uploads/%s", filename)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url": fileURL,
	})
}