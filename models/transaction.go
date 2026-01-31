package models

// Header Transaksi (Tabel transactions)
type Transaction struct {
	ID            int     `json:"id"`
	OrderCode     string  `json:"order_code"`
	CustomerName  string  `json:"customer_name"`
	CustomerPhone string  `json:"customer_phone"`
	Address       string  `json:"customer_address"`
	PaymentMethod string  `json:"payment_method"`
	TotalAmount   float64 `json:"total_amount"`
	Status        int     `json:"status"`
	Date          string  `json:"date"` // Kita ambil dari created_at
	
	// Optional: Untuk menerima input detail saat checkout
	Details []TransactionDetail `json:"details,omitempty"` 
}

// Detail Item (Tabel transaction_details)
type TransactionDetail struct {
	ID            int     `json:"id"`
	TransactionID int     `json:"transaction_id"`
	BookID        int     `json:"book_id"`
	Quantity      int     `json:"quantity"`
	Price         float64 `json:"price"` // Harga saat beli
	Book struct {
		Title string `json:"title"`
		Image string `json:"image"`
	} `json:"book"`
}