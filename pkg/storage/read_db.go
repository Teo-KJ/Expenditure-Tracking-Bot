package storage

import (
	"database/sql"
	"fmt"
	"log"
	"main/pkg/transaction"
	"strings"
)

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

// GetTotalAmountByPaidForFamily retrieves the total sum of amounts grouped by the 'paid_for_family' status.
func GetTotalAmountByPaidForFamily() (map[bool]float32, error) {
	querySQL := `
		SELECT
			paid_for_family,
			SUM(amount) AS total_amount
		FROM
			transactions
		GROUP BY
			paid_for_family
		ORDER BY
			paid_for_family;
	`
	amountByPaidForFamily := make(map[bool]float32)

	currentDB, err := GetDB()
	if err != nil {
		log.Printf("Error getting DB connection for 'paid_for_family' totals: %v", err)
		return nil, fmt.Errorf("failed to get DB connection: %w", err)
	}

	rows, err := currentDB.Query(querySQL)
	if err != nil {
		log.Printf("Error querying transaction totals by 'paid_for_family': %v (SQL: %s)", err, querySQL)
		return nil, fmt.Errorf("database query for 'paid_for_family' totals failed: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Printf("Error closing rows for 'paid_for_family' totals: %v", err)
		}
	}(rows)

	for rows.Next() {
		var paidForFamilyStatus bool
		var totalAmount float32
		// Handle potential NULL values for SUM(amount) if a group has no non-NULL amounts,
		// though SUM usually returns 0 for empty groups or NULL if all inputs are NULL.
		// Using sql.NullFloat64 might be more robust if SUM can return NULL.
		// For simplicity, assuming SUM(amount) will return a float32 (or 0).
		if err := rows.Scan(&paidForFamilyStatus, &totalAmount); err != nil {
			log.Printf("Error scanning 'paid_for_family' total row: %v", err)
			return nil, fmt.Errorf("failed to scan 'paid_for_family' total row: %w", err)
		}
		amountByPaidForFamily[paidForFamilyStatus] = totalAmount
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating 'paid_for_family' total rows: %v", err)
		return nil, fmt.Errorf("error during 'paid_for_family' total row iteration: %w", err)
	}

	log.Printf("Successfully retrieved transaction totals by 'paid_for_family' status for %d groups.", len(amountByPaidForFamily))
	return amountByPaidForFamily, nil
}

// GetTotalAmountByIsClaimable retrieves the total sum of amounts grouped by the 'is_claimable' status.
func GetTotalAmountByIsClaimable() (map[bool]float32, error) {
	querySQL := `
		SELECT
			is_claimable,
			SUM(amount) AS total_amount
		FROM
			transactions
		GROUP BY
			is_claimable
		ORDER BY
			is_claimable;
	`
	amountByIsClaimable := make(map[bool]float32)

	currentDB, err := GetDB()
	if err != nil {
		log.Printf("Error getting DB connection for 'is_claimable' totals: %v", err)
		return nil, fmt.Errorf("failed to get DB connection: %w", err)
	}

	rows, err := currentDB.Query(querySQL)
	if err != nil {
		log.Printf("Error querying transaction totals by 'is_claimable': %v (SQL: %s)", err, querySQL)
		return nil, fmt.Errorf("database query for 'is_claimable' totals failed: %w", err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Printf("Error closing rows for 'is_claimable' totals: %v", err)
		}
	}(rows)

	for rows.Next() {
		var isClaimableStatus bool
		var totalAmount float32
		if err := rows.Scan(&isClaimableStatus, &totalAmount); err != nil {
			log.Printf("Error scanning 'is_claimable' total row: %v", err)
			return nil, fmt.Errorf("failed to scan 'is_claimable' total row: %w", err)
		}
		amountByIsClaimable[isClaimableStatus] = totalAmount
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error iterating 'is_claimable' total rows: %v", err)
		return nil, fmt.Errorf("error during 'is_claimable' total row iteration: %w", err)
	}

	log.Printf("Successfully retrieved transaction totals by 'is_claimable' status for %d groups.", len(amountByIsClaimable))
	return amountByIsClaimable, nil
}
