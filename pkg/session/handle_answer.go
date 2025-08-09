package session

import (
	"fmt"
	"main/pkg/transaction"
)

// HandleAnswer processes the user's answer, updates the session,
// applies autofill logic, and skips already answered questions.
func (s *UserSession) HandleAnswer(answer string) error {
	var err error

	switch s.CurrentQuestion {
	case QuestionName:
		s.Answers.Name = answer
	case QuestionAmount:
		s.Answers.Amount, err = transaction.ValidateAmount(answer)
		if err != nil {
			// Return a user-friendly error message
			return fmt.Errorf("invalid amount: %w. Please enter a valid number", err)
		}
	case QuestionCurrency:
		// TODO: Consider adding validation for currency (e.g., check if 'answer' is in 'Currencies' list)
		s.Answers.Currency = answer
	case QuestionDate:
		s.Answers.Date = transaction.ProcessDate(answer)
	case QuestionIsClaimable:
		s.Answers.IsClaimable, err = transaction.ValidateBool(answer)
		if err != nil {
			return fmt.Errorf("invalid input for 'claimable': %w. Please answer 'yes' or 'no'", err)
		}
	case QuestionPaidForFamily:
		s.Answers.PaidForFamily, err = transaction.ValidateBool(answer)
		if err != nil {
			return fmt.Errorf("invalid input for 'paid for family': %w. Please answer 'yes' or 'no'", err)
		}
	case QuestionCategory:
		// TODO: Consider adding validation for category (e.g., check if 'answer' is in 'TransactionCategory' list)
		s.Answers.Category = answer
	default:
		// This state should ideally not be reached if IsSessionComplete is checked before calling HandleAnswer.
		return fmt.Errorf("invalid question number: %d", s.CurrentQuestion)
	}

	// If an error occurred during the specific answer processing (e.g., validation failed),
	// return the error. s.CurrentQuestion is NOT advanced, so the same question will be asked again.
	if err != nil {
		return err
	}

	// Advance to what would normally be the next question index.
	s.CurrentQuestion++

	return nil
}
