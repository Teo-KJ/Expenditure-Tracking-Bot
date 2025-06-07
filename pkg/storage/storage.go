package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"main/pkg/transaction" // Assuming Transaction is here
	"os"
	"strings" // Needed for joining WHERE clauses
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
// with optional filtering by category, is_claimable, and paid_for_family.
func GetAllTransactionsFromDB(categoryFilter string, isClaimableFilter *bool, paidForFamilyFilter *bool) ([]transaction.Transaction, error) {
	baseSelectSQL := `
        SELECT id, name, amount, currency, date, is_claimable, paid_for_family, category, created_at
        FROM transactions
        `
	var conditions []string
	var args []interface{}
	argID := 1

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
	}

	finalSQL := baseSelectSQL
	if len(conditions) > 0 {
		finalSQL += " WHERE " + strings.Join(conditions, " AND ")
	}
	finalSQL += " ORDER BY date DESC, created_at DESC;" // Keep existing order

	var transactions []transaction.Transaction

	currentDB, err := GetDB()
	if err != nil {
		log.Printf("Error getting DB connection for select: %v", err)
		return nil, fmt.Errorf("failed to get DB connection: %w", err)
	}

	rows, err := currentDB.Query(finalSQL, args...)
	if err != nil {
		log.Printf("Error querying transactions from database with filters: %v (SQL: %s, Args: %v)", err, finalSQL, args)
		return nil, fmt.Errorf("database query failed: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Printf("Error closing DB: %v", err)
		}
	}(rows)

	for rows.Next() {
		var t transaction.Transaction
		err := rows.Scan(
			&t.ID,
			&t.Name,
			&t.Amount,
			&t.Currency,
			&t.Date,
			&t.IsClaimable,
			&t.PaidForFamily,
			&t.Category,
			&t.CreatedAt,
		)
		if err != nil {
			log.Printf("Error scanning transaction row: %v", err)
			return nil, fmt.Errorf("failed to scan transaction row: %w", err)
		}
		transactions = append(transactions, t)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating transaction rows: %v", err)
		return nil, fmt.Errorf("error during row iteration: %w", err)
	}

	log.Printf("Successfully retrieved %d transactions from the database with applied filters.", len(transactions))
	return transactions, nil
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
