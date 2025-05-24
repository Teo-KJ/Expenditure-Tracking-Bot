package main

import (
	"encoding/json" // To work with JSON data
	"fmt"
	"gopkg.in/yaml.v3"
	"log"
	"main/pkg/config"
	"main/pkg/storage"
	"net/http" // The core HTTP package
	"os"       // To potentially read port from environment
	"strconv"  // To parse boolean query parameters
	"time"     // For example data
)

// Define a struct for example JSON responses
type HealthStatus struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

// --- Handler Functions ---

// healthCheckHandler responds with the server's status.
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests for this endpoint
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	status := HealthStatus{
		Status:    "OK",
		Timestamp: time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err := json.NewEncoder(w).Encode(status)
	if err != nil {
		log.Printf("Error encoding health status JSON: %v", err)
	}
	log.Printf("Served %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
}

// exampleHandler provides a simple example endpoint.
func exampleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	response := MessageResponse{
		Message: "Hello from the Go backend!",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("Error encoding example response JSON: %v", err)
	}
	log.Printf("Served %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
}

// getTransactionsHandler retrieves transactions, allowing filtering via query parameters.
func getTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get query parameters
	queryParams := r.URL.Query()

	categoryFilter := queryParams.Get("category") // Returns "" if not present

	var isClaimableFilter *bool
	if claimableStr := queryParams.Get("is_claimable"); claimableStr != "" {
		val, err := strconv.ParseBool(claimableStr)
		if err != nil {
			log.Printf("Invalid boolean value for is_claimable: %s. Error: %v", claimableStr, err)
			http.Error(w, "Invalid value for 'is_claimable' parameter. Use 'true' or 'false'.", http.StatusBadRequest)
			return
		}
		isClaimableFilter = &val
	}

	var paidForFamilyFilter *bool
	if paidForFamilyStr := queryParams.Get("paid_for_family"); paidForFamilyStr != "" {
		val, err := strconv.ParseBool(paidForFamilyStr)
		if err != nil {
			log.Printf("Invalid boolean value for paid_for_family: %s. Error: %v", paidForFamilyStr, err)
			http.Error(w, "Invalid value for 'paid_for_family' parameter. Use 'true' or 'false'.", http.StatusBadRequest)
			return
		}
		paidForFamilyFilter = &val
	}

	log.Printf("Fetching transactions with filters - Category: '%s', IsClaimable: %v, PaidForFamily: %v",
		categoryFilter, isClaimableFilter, paidForFamilyFilter)

	// Fetch data from storage using the filters
	// Assuming storage.GetAllTransactionsFromDB now accepts these filter parameters
	transactions, err := storage.GetAllTransactionsFromDB(categoryFilter, isClaimableFilter, paidForFamilyFilter)
	if err != nil {
		log.Printf("Error fetching transactions: %v", err)
		// Consider more specific error mapping if storage layer provides it
		http.Error(w, "Internal Server Error while fetching transactions.", http.StatusInternalServerError)
		return
	}

	// --- Respond with JSON ---
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(transactions)
	if err != nil {
		// This error is tricky because headers might have already been sent.
		// Log it, but avoid writing another http.Error if possible.
		log.Printf("Error encoding transactions JSON: %v", err)
	}
	log.Printf("Served %s %s with %d transactions from %s", r.Method, r.URL.Path, len(transactions), r.RemoteAddr)
}

// --- Main Function ---

func main() {
	yamlFile, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("Error reading YAML file: %v", err)
	}

	var cfg config.Config
	err = yaml.Unmarshal(yamlFile, &cfg)
	if err != nil {
		log.Fatalf("Error unmarshalling YAML: %v", err)
	}

	if storage.UseDBToSave {
		// Initialize Database
		// Assuming storage.InitDB takes config.DatabaseConfig
		err = storage.InitDB(cfg.Database)
		if err != nil {
			log.Fatalf("Failed to initialize database: %v", err) // Changed from Panic to Fatalf for consistency
		}
		defer storage.CloseDB()
		log.Println("Database initialized successfully.")
	}

	// --- Register Handlers ---
	http.HandleFunc("/health", healthCheckHandler)
	http.HandleFunc("/api/v1/example", exampleHandler)
	http.HandleFunc("/api/v1/transactions", getTransactionsHandler) // This now supports query params

	// --- Configure and Start Server ---
	port := "8080"
	serverAddr := fmt.Sprintf(":%s", port)

	log.Printf("Starting HTTP server on %s", serverAddr)

	err = http.ListenAndServe(serverAddr, nil)
	if err != nil {
		log.Fatalf("HTTP server failed to start: %v", err)
	}
}
