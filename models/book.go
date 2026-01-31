package models

// Sesuaikan JSON tag dengan apa yang Frontend kirim/terima
type Book struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	Author      string  `json:"author"`
	Price       float64 `json:"price"`
	Category    string  `json:"category"`
	Stock       int     `json:"stock"`
	ImageURL    string  `json:"image"` // Di DB kolomnya image_url, di JSON kita sebut image
	Description string  `json:"description"`
}