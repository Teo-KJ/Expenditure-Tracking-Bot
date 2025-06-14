package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"main/pkg/transaction" // Assuming Transaction is here
	"os"
	"strings"
)

// SaveFilePath File to save responses
const SaveFilePath = "responses.txt"

// SaveResponseToFile saves the transaction to a file.
func SaveResponseToFile(response transaction.Transaction) { // Assuming Transaction is an older version
	file, err := os.OpenFile(SaveFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Error opening file: %v", err)
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Printf("Error closing file: %v", err)
		}
	}(file)

	data, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error marshaling response: %v", err)
		return
	}

	_, err = file.WriteString(fmt.Sprintf("%s\n", data))
	if err != nil {
		log.Printf("Error writing to file: %v", err)
	}
}

// SaveTransactionToDB saves the transaction to the database.
func SaveTransactionToDB(response transaction.Transaction) error {
	insertSQL := `
        INSERT INTO transactions (name, amount, currency, date, is_claimable, paid_for_family, category)
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING id;
        `
	var insertedID int64

	currentDB, err := GetDB()
	if err != nil {
		log.Printf("Error getting DB connection for insert: %v", err)
		return fmt.Errorf("failed to get DB connection: %w", err)
	}

	err = currentDB.QueryRow(
		insertSQL,
		response.Name,
		response.Amount,
		response.Currency,
		response.Date,
		response.IsClaimable,
		response.PaidForFamily,
		response.Category,
	).Scan(&insertedID)

	if err != nil {
		log.Printf("Error inserting transaction into database: %v", err)
		return fmt.Errorf("database insert failed: %w", err)
	}

	log.Printf("Successfully inserted transaction with ID: %d", insertedID)
	return nil
}

// GetAllTransactionsFromDB retrieves transactions from the database,
// with optional filtering by category, is_claimable, paid_for_family,
// and supports pagination.
// It returns the slice of transactions for the current page,
// the total number of items matching the filters (before pagination), and an error.
func GetAllTransactionsFromDB(
	categoryFilter string,
	isClaimableFilter *bool,
	paidForFamilyFilter *bool,
	page int, // Current page number (1-based)
	limit int, // Number of items per page
) ([]transaction.Transaction, int, error) { // Returns: transactions, totalItems, error
	currentDB, err := GetDB() // Assuming GetDB() returns your *sql.DB or *sqlx.DB
	if err != nil {
		log.Printf("Error getting DB connection: %v", err)
		return nil, 0, fmt.Errorf("failed to get DB connection: %w", err)
	}

	var conditions []string
	var args []interface{}
	argID := 1 // For SQL query placeholder numbering ($1, $2, etc.)

	// Build WHERE clause and arguments for filtering
	if categoryFilter != "" {
		conditions = append(conditions, fmt.Sprintf("category = $%d", argID))
		args = append(args, categoryFilter)
		argID++
	}
	if isClaimableFilter != nil {
		conditions = append(conditions, fmt.Sprintf("is_claimable = $%d", argID))
		args = append(args, *isClaimableFilter)
		argID++
	}
	if paidForFamilyFilter != nil {
		conditions = append(conditions, fmt.Sprintf("paid_for_family = $%d", argID))
		args = append(args, *paidForFamilyFilter)
		argID++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	// --- Query 1: Get the total count of items matching the filters ---
	countSQL := `SELECT COUNT(*) FROM transactions` + whereClause
	var totalItems int
	err = currentDB.QueryRow(countSQL, args...).Scan(&totalItems) // Use the same filter args
	if err != nil {
		log.Printf("Error querying total transaction count: %v (SQL: %s, Args: %v)", err, countSQL, args)
		return nil, 0, fmt.Errorf("database query for total count failed: %w", err)
	}

	// If no items match the filter, return early
	if totalItems == 0 {
		return []transaction.Transaction{}, 0, nil
	}

	// --- Query 2: Get the paginated list of transactions ---
	selectSQL := `
        SELECT id, name, amount, currency, date, is_claimable, paid_for_family, category, created_at
        FROM transactions
    ` + whereClause + ` ORDER BY date DESC, created_at DESC` // Keep existing order

	// Add LIMIT and OFFSET for pagination
	// The placeholders for LIMIT and OFFSET will be after the filter args
	selectSQL += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argID, argID+1)

	offset := (page - 1) * limit
	// Append limit and offset to the arguments list for the main query
	queryArgs := append(args, limit, offset)

	rows, err := currentDB.Query(selectSQL, queryArgs...)
	if err != nil {
		log.Printf("Error querying paginated transactions: %v (SQL: %s, Args: %v)", err, selectSQL, queryArgs)
		return nil, 0, fmt.Errorf("database query for paginated transactions failed: %w", err)
	}
	defer func(rows *sql.Rows) {
		err = rows.Close()
		if err != nil {
			log.Printf("Error closing rows for getting all transactions: %v", err)
		}
	}(rows)

	var transactions []transaction.Transaction
	for rows.Next() {
		var t transaction.Transaction
		// Ensure the Scan arguments match the columns in your SELECT statement
		err := rows.Scan(
			&t.ID, &t.Name, &t.Amount, &t.Currency, &t.Date,
			&t.IsClaimable, &t.PaidForFamily, &t.Category, &t.CreatedAt,
		)
		if err != nil {
			log.Printf("Error scanning transaction row: %v", err)
			// Decide if you want to return immediately or try to process other rows
			return nil, 0, fmt.Errorf("failed to scan transaction row: %w", err)
		}
		transactions = append(transactions, t)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating transaction rows: %v", err)
		return nil, 0, fmt.Errorf("error during row iteration: %w", err)
	}

	log.Printf("Successfully retrieved %d transactions (page %d, limit %d, total matching %d).",
		len(transactions), page, limit, totalItems)
	return transactions, totalItems, nil
}

// GetTransactionCountByCategory retrieves the total number of transactions for each category.
func GetTransactionCountByCategory() (map[string]float32, error) {
	querySQL := `
		SELECT
			category,
			SUM(amount) AS total_cost
		FROM
			transactions
		GROUP BY
			category
		ORDER BY
			total_cost DESC;
    `
	categoryCounts := make(map[string]float32)

	currentDB, err := GetDB()
	if err != nil {
		log.Printf("Error getting DB connection for category count: %v", err)
		return nil, fmt.Errorf("failed to get DB connection: %w", err)
	}

	rows, err := currentDB.Query(querySQL)
	if err != nil {
		log.Printf("Error querying transaction counts by category: %v (SQL: %s)", err, querySQL)
		return nil, fmt.Errorf("database query for category counts failed: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Printf("Error closing rows for category count: %v", err)
		}
	}(rows)

	for rows.Next() {
		var category string
		var count float32
		if err := rows.Scan(&category, &count); err != nil {
			log.Printf("Error scanning category count row: %v", err)
			return nil, fmt.Errorf("failed to scan category count row: %w", err)
		}
		categoryCounts[category] = count
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating category count rows: %v", err)
		return nil, fmt.Errorf("error during category count row iteration: %w", err)
	}

	log.Printf("Successfully retrieved transaction counts for %d categories.", len(categoryCounts))
	return categoryCounts, nil
}
