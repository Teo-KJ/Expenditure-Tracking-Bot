package main

import (
	"encoding/json" // To work with JSON data
	"fmt"
	"github.com/patrickmn/go-cache"
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
	"time"
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

// --- Global Cache ---
var c = cache.New(5*time.Minute, 10*time.Minute)

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

// getPrefilledExpensesHandler creates an HTTP handler that serves the list
// of pre-filled expenses from the configuration.
func getPrefilledExpensesHandler(expenses []config.FrequentExpense) http.HandlerFunc {
	// Cache the response indefinitely since it's based on config
	prefilledExpensesJSON, err := json.Marshal(expenses)
	if err != nil {
		// This would be a startup-time error, so logging it fatally is reasonable.
		log.Fatalf("Failed to marshal pre-filled expenses: %v", err)
	}
	c.Set("prefilled-expenses", prefilledExpensesJSON, cache.NoExpiration)

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}

		// Serve from cache
		if cachedJSON, found := c.Get("prefilled-expenses"); found {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, err = w.Write(cachedJSON.([]byte))
			if err != nil {
				return
			}
			log.Printf("Served %s %s from cache", r.Method, r.URL.Path)
			return
		}

		// Fallback (should not happen if cache is pre-warmed)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err = json.NewEncoder(w).Encode(expenses)
		if err != nil {
			return
		}
	}
}

// getTransactionsHandler retrieves transactions, allowing filtering and pagination.
func getTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	cacheKey := r.URL.String() // Use the full URL as the cache key

	// Check cache first
	if cachedResponse, found := c.Get(cacheKey); found {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(cachedResponse)
		if err != nil {
			return
		}
		log.Printf("Served %s %s from cache", r.Method, r.URL.Path)
		return
	}

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
	}

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

	// Store in cache
	c.Set(cacheKey, response, cache.DefaultExpiration)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		return
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

	if err := storage.InsertTransaction(newTransaction); err != nil {
		log.Printf("Error inserting transaction: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Invalidate cache
	c.Flush()
	log.Println("Cache flushed due to new transaction")

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
	mux.HandleFunc("/health", healthCheckHandler)
	mux.HandleFunc("/api/v1/transactions", transactionsHandler)

	prefilledHandler := getPrefilledExpensesHandler(cfg.FrequentExpenses)
	mux.HandleFunc("/api/v1/prefilled-expenses", prefilledHandler)

	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins: []string{"http://localhost:3000", "http://192.168.1.11:3000"},
		AllowOriginFunc: func(origin string) bool {
			match, _ := regexp.MatchString(`^https?://(localhost|127\.0\.0\.1|192\.168\.\d{1,3}\.\d{1,3}|10\.\d{1,3}\.\d{1,3}\.\d{1,3}|172\.(1[6-9]|2[0-9]|3[0-1])\.\d{1,3}\.\d{1,3}):\d+$`, origin)
			return match
		},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
	})

	handler := corsMiddleware.Handler(mux)

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
