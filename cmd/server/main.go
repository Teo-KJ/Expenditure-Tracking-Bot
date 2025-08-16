package main

import (
	"encoding/json" // To work with JSON data
	"fmt"
	"github.com/rs/cors"
	"gopkg.in/yaml.v3"
	"log"
	"main/pkg/config"
	"main/pkg/storage"     // Assuming your storage functions are here
	"main/pkg/transaction" // Assuming your Transaction struct is here
	"net/http"             // The core HTTP package
	"os"                   // To potentially read port from environment
	"regexp"
	"strconv" // To parse boolean and integer query parameters
	"time"    // For example data
)

// Define a struct for example JSON responses
type HealthStatus struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

// PaginatedTransactionsResponse defines the structure for paginated transaction results.
type PaginatedTransactionsResponse struct {
	Transactions []transaction.Transaction `json:"transactions"`
	CurrentPage  int                       `json:"currentPage"`
	PageSize     int                       `json:"pageSize"`
	TotalItems   int                       `json:"totalItems"`
	TotalPages   int                       `json:"totalPages"`
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

// getTransactionsHandler retrieves transactions, allowing filtering and pagination.
func getTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()

	// Filtering parameters
	categoryFilter := queryParams.Get("category")

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

	// Pagination parameters
	pageStr := queryParams.Get("page")
	limitStr := queryParams.Get("limit")

	page := 1   // Default page
	limit := 10 // Default limit (items per page)
	var err error

	if pageStr != "" {
		page, err = strconv.Atoi(pageStr)
		if err != nil || page < 1 {
			log.Printf("Invalid value for 'page' parameter: %s. Must be a positive integer.", pageStr)
			http.Error(w, "Invalid value for 'page' parameter. Must be a positive integer.", http.StatusBadRequest)
			return
		}
	}

	if limitStr != "" {
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit < 1 {
			log.Printf("Invalid value for 'limit' parameter: %s. Must be a positive integer.", limitStr)
			http.Error(w, "Invalid value for 'limit' parameter. Must be a positive integer.", http.StatusBadRequest)
			return
		}
		// Optional: You might want to set a maximum limit
		// if limit > 100 {
		// 	limit = 100 // Cap the limit to prevent abuse
		// }
	}

	log.Printf("Fetching transactions with filters - Category: '%s', IsClaimable: %v, PaidForFamily: %v, Page: %d, Limit: %d",
		categoryFilter, isClaimableFilter, paidForFamilyFilter, page, limit)

	// Fetch data from storage using the filters and pagination parameters
	// storage.GetAllTransactionsFromDB will need to be updated to return totalItems as well.
	transactions, totalItems, err := storage.GetAllTransactionsFromDB(
		categoryFilter,
		isClaimableFilter,
		paidForFamilyFilter,
		page,
		limit,
	)
	if err != nil {
		log.Printf("Error fetching transactions: %v", err)
		http.Error(w, "Internal Server Error while fetching transactions.", http.StatusInternalServerError)
		return
	}

	// Calculate total pages
	totalPages := 0
	if totalItems > 0 && limit > 0 {
		totalPages = (totalItems + limit - 1) / limit // Ceiling division
	}

	response := PaginatedTransactionsResponse{
		Transactions: transactions,
		CurrentPage:  page,
		PageSize:     limit,
		TotalItems:   totalItems,
		TotalPages:   totalPages,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("Error encoding transactions JSON: %v", err)
	}
	log.Printf("Served %s %s with %d transactions (page %d, limit %d, total %d) from %s",
		r.Method, r.URL.Path, len(transactions), page, limit, totalItems, r.RemoteAddr)
}

// createTransactionHandler handles the creation of a new transaction.
func createTransactionHandler(w http.ResponseWriter, r *http.Request) {
	var newTransaction transaction.Transaction
	if err := json.NewDecoder(r.Body).Decode(&newTransaction); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Insert the new transaction into the database
	if err := storage.InsertTransaction(newTransaction); err != nil {
		log.Printf("Error inserting transaction: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	response := MessageResponse{Message: "Transaction created successfully"}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response JSON: %v", err)
	}
}

// transactionsHandler routes to different handlers based on the HTTP method.
func transactionsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getTransactionsHandler(w, r)
	case http.MethodPost:
		createTransactionHandler(w, r)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
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

	if cfg.FeaturesConfig.SaveToDB {
		// Initialize Database
		// Assuming storage.InitDB takes config.DatabaseConfig
		err = storage.InitDB(cfg.Database)
		if err != nil {
			log.Fatalf("Failed to initialize database: %v", err)
		}
		defer storage.CloseDB()
		log.Println("Database initialized successfully.")
	} else {
		log.Println("Database not configured. Transactions API might not function as expected if DB is required.")
	}

	mux := http.NewServeMux()
	// --- Register Handlers ---
	mux.HandleFunc("/health", healthCheckHandler)
	mux.HandleFunc("/api/v1/example", exampleHandler)
	mux.HandleFunc("/api/v1/transactions", transactionsHandler)

	// --- CORS Middleware ---
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"http://localhost:3000", "http://192.168.1.11:3000"},
		AllowOriginFunc: func(origin string) bool {
			match, _ := regexp.MatchString(`^https?://(localhost|127\.0\.0\.1|192\.168\.\d{1,3}\.\d{1,3}|10\.\d{1,3}\.\d{1,3}\.\d{1,3}|172\.(1[6-9]|2[0-9]|3[0-1])\.\d{1,3}\.\d{1,3}):\d+$`, origin)
			return match
		},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	})

	handler := c.Handler(mux)

	// --- Configure and Start Server ---
	port := "8081"
	serverAddr := fmt.Sprintf(":%s", port)

	log.Printf("Starting HTTP server on %s", serverAddr)

	log.Printf("The available endpoints:\nHealth check endpoint: localhost:%v/health\nTransactions API endpoint: localhost:%v/api/v1/transactions",
		port, port)

	err = http.ListenAndServe(serverAddr, handler)
	if err != nil {
		log.Fatalf("HTTP server failed to start: %v", err)
	}
}
