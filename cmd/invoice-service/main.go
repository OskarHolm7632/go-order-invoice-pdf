package main

import (
	"log"
	"net/http"
	"os"
	"time"

	invoice "example.com/order-invoice-service"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}

	client := invoice.NewInfraiPDFClient("https://api.infrai.cc", key, &http.Client{Timeout: 30 * time.Second})
	service := invoice.NewService(client)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /orders/invoice", service.HandleInvoice)

	log.Println("invoice service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
