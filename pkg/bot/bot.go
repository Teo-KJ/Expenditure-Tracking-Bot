package bot

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"main/pkg/config"
	"main/pkg/session"
	"main/pkg/storage"
	"strings"
)

const (
	addOption                 = "/add"
	transactionsSummaryOption = "/summary"
)

// Map to track ongoing sessions (active users)
var userSessions = make(map[int64]*session.UserSession)

// Bot represents the Telegram bot.
type Bot struct {
	api     *tgbotapi.BotAPI
	expense []config.FrequentExpense
}

// NewBot creates a new bot instance.
func NewBot(token string, preFilledExpense []config.FrequentExpense) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}
	api.Debug = true
	log.Printf("Authorized on account %s", api.Self.UserName)
	return &Bot{api: api, expense: preFilledExpense}, nil
}

// StartListening starts listening for updates.
func (b *Bot) StartListening(userSessions map[int64]*session.UserSession) {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			err := b.handleTextMessage(update.Message, userSessions)
			if err != nil {
				log.Println(err)
			}
		} else if update.CallbackQuery != nil {
			err := b.handleCallbackQuery(update.CallbackQuery, userSessions)
			if err != nil {
				log.Println(err)
			}
		}
	}
}

// handleTextMessage handles incoming text messages.
func (b *Bot) handleTextMessage(message *tgbotapi.Message, userSessions map[int64]*session.UserSession) error {
	chatID := message.Chat.ID

	switch message.Text {
	case addOption:
		log.Printf("Chat %v: Received %v command", chatID, addOption)
		return b.startSession(chatID, userSessions)

	case transactionsSummaryOption: // It's good practice to have a cancel command
		log.Printf("Chat %v: Received %v command", chatID, transactionsSummaryOption)

		categoryCounts, err := storage.GetTransactionCountByCategory()
		if err != nil {
			log.Printf("Chat %d: Error getting transaction summary: %v", chatID, err)
			// Send a generic error message to the user

			errMsg := tgbotapi.NewMessage(chatID, "Sorry, I couldn't retrieve the transaction summary at this time. Please try again later.")
			_, sendErr := b.api.Send(errMsg)
			if sendErr != nil {
				log.Printf("Chat %d: Error sending summary error message: %v", chatID, sendErr)
			}

			err = b.sendDefaultMessage(chatID)
			if err != nil {
				log.Printf("Chat %d: Error sending summary error message: %v", chatID, sendErr)
			}

			return err // Return the original error
		}
		var summaryMessageBuilder strings.Builder
		summaryMessageBuilder.WriteString("Transaction Summary by Category:")

		if len(categoryCounts) == 0 {
			summaryMessageBuilder.WriteString("\nNo transactions found.")
		} else {
			totalExpense := float32(0)
			for category, count := range categoryCounts {
				summaryMessageBuilder.WriteString(fmt.Sprintf("\n- %v: %v", category, count))
				totalExpense += count
			}
			summaryMessageBuilder.WriteString(fmt.Sprintf("\n- Total expense: %v", totalExpense))
		}

		summaryMessageBuilder.WriteString("\n\nTotal claimable:")
		amountByIsClaimable, errGetIsClaimable := storage.GetTotalAmountByIsClaimable()
		if errGetIsClaimable != nil {
			log.Printf("Chat %v: Error getting 'is_claimable' totals: %v", chatID, err)
		}
		if len(categoryCounts) == 0 || errGetIsClaimable != nil {
			summaryMessageBuilder.WriteString("\nNo transactions found.")
		} else {
			for category, count := range amountByIsClaimable {
				summaryMessageBuilder.WriteString(fmt.Sprintf("\n- %v: %v", category, count))
			}
		}

		summaryMessageBuilder.WriteString("\n\nTotal paid for family:")
		amountByPaidForFamily, errPaidByFamily := storage.GetTotalAmountByPaidForFamily()
		if errPaidByFamily != nil {
			log.Printf("Chat %v: Error getting 'paid_for_family' totals: %v", chatID, err)
		}
		if len(amountByPaidForFamily) == 0 || errPaidByFamily != nil {
			summaryMessageBuilder.WriteString("\nNo transactions found.")
		} else {
			for category, count := range amountByPaidForFamily {
				summaryMessageBuilder.WriteString(fmt.Sprintf("\n- %v: %v", category, count))
			}
		}

		msg := tgbotapi.NewMessage(chatID, summaryMessageBuilder.String())
		_, sendErr := b.api.Send(msg)
		if sendErr != nil {
			log.Printf("Chat %d: Error sending summary message: %v", chatID, sendErr)
		}
		return sendErr

	default:
		if _, exists := userSessions[chatID]; exists {
			return b.handleAnswer(message, userSessions)
		}

		return b.sendDefaultMessage(chatID)
	}
}

// startSession starts a new session for the user.
func (b *Bot) startSession(chatID int64, userSessions map[int64]*session.UserSession) error {
	userSessions[chatID] = session.NewUserSession()
	return b.askCurrentQuestion(chatID, userSessions)
}

// askCurrentQuestion sends the current question to the user, prepended with a summary of previous answers.
func (b *Bot) askCurrentQuestion(chatID int64, userSessions map[int64]*session.UserSession) error {
	userSession := userSessions[chatID]

	var messageBuilder strings.Builder

	// Build a summary of answers provided so far, but not for the very first question.
	if userSession.CurrentQuestion > session.QuestionName {
		answers := userSession.Answers
		var summaryParts []string

		// Check which answers have been provided and add them to the summary.
		if answers.Name != "" {
			// Escape user-provided text to prevent them from breaking Markdown formatting.
			summaryParts = append(summaryParts, fmt.Sprintf("*Name:* %s", tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, answers.Name)))
		}
		if answers.Amount > 0 {
			summaryParts = append(summaryParts, fmt.Sprintf("*Amount:* `%.2f`", answers.Amount))
		}
		if answers.Currency != "" {
			summaryParts = append(summaryParts, fmt.Sprintf("*Currency:* %s", tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, answers.Currency)))
		}
		// FIX: Check if a time.Time object is set, and format it.
		if answers.Date != "" {
			summaryParts = append(summaryParts, fmt.Sprintf("*Date:* %s", tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, answers.Date)))
		}
		// For booleans, we check if the question has been passed in the session flow.
		if userSession.CurrentQuestion > session.QuestionIsClaimable {
			summaryParts = append(summaryParts, fmt.Sprintf("*Claimable:* %t", answers.IsClaimable))
		}
		if userSession.CurrentQuestion > session.QuestionPaidForFamily {
			summaryParts = append(summaryParts, fmt.Sprintf("*Paid for Family:* %t", answers.PaidForFamily))
		}
		if answers.Category != "" { // Category can be auto-filled
			summaryParts = append(summaryParts, fmt.Sprintf("*Category:* %s", tgbotapi.EscapeText(tgbotapi.ModeMarkdownV2, answers.Category)))
		}

		if len(summaryParts) > 0 {
			messageBuilder.WriteString("*Your progress so far:*\n")
			messageBuilder.WriteString(strings.Join(summaryParts, "\n"))
			// FIX: Escape the hyphens for the separator line.
			messageBuilder.WriteString("\n")
		}
	}

	question := session.Questions[userSession.CurrentQuestion]
	messageBuilder.WriteString(question)

	msg := tgbotapi.NewMessage(chatID, messageBuilder.String())
	// Set the ParseMode to render the Markdown formatting (bolding, code blocks).
	msg.ParseMode = tgbotapi.ModeMarkdownV2

	// ... (The rest of your keyboard logic remains the same) ...
	if userSession.CurrentQuestion == session.QuestionName {
		var keyboardRows [][]tgbotapi.InlineKeyboardButton // Slice of rows

		// Iterate through TransactionCategory, taking two items at a time
		for i := 0; i < len(b.expense); i += 2 {
			// Create the first button for the row
			button1 := tgbotapi.NewInlineKeyboardButtonData(b.expense[i].Name, b.expense[i].Name)

			var rowButtons []tgbotapi.InlineKeyboardButton
			rowButtons = append(rowButtons, button1)

			// Check if there's a second item for this row
			if i+1 < len(b.expense) {
				button2 := tgbotapi.NewInlineKeyboardButtonData(b.expense[i+1].Name, b.expense[i+1].Name)
				rowButtons = append(rowButtons, button2)
			}

			// Add the current row (with one or two buttons) to our list of rows
			keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(rowButtons...))
		}

		if len(keyboardRows) > 0 {
			keyboard := tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)
			msg.ReplyMarkup = keyboard
		}
	}

	if userSession.CurrentQuestion == session.QuestionIsClaimable || userSession.CurrentQuestion == session.QuestionPaidForFamily {
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Yes", "yes"),
				tgbotapi.NewInlineKeyboardButtonData("No", "no"),
			),
		)
		msg.ReplyMarkup = keyboard
	}

	if userSession.CurrentQuestion == session.QuestionCurrency {
		var currencyButtons []tgbotapi.InlineKeyboardButton
		for _, currency := range session.Currencies {
			currencyButtons = append(currencyButtons, tgbotapi.NewInlineKeyboardButtonData(currency, currency))
		}
		keyboard := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(currencyButtons...))
		msg.ReplyMarkup = keyboard
	}

	if userSession.CurrentQuestion == session.QuestionCategory {
		var keyboardRows [][]tgbotapi.InlineKeyboardButton // Slice of rows

		// Iterate through TransactionCategory, taking two items at a time
		for i := 0; i < len(session.TransactionCategory); i += 2 {
			// Create the first button for the row
			button1 := tgbotapi.NewInlineKeyboardButtonData(session.TransactionCategory[i], session.TransactionCategory[i])

			var rowButtons []tgbotapi.InlineKeyboardButton
			rowButtons = append(rowButtons, button1)

			// Check if there's a second item for this row
			if i+1 < len(session.TransactionCategory) {
				button2 := tgbotapi.NewInlineKeyboardButtonData(session.TransactionCategory[i+1], session.TransactionCategory[i+1])
				rowButtons = append(rowButtons, button2)
			}

			// Add the current row (with one or two buttons) to our list of rows
			keyboardRows = append(keyboardRows, tgbotapi.NewInlineKeyboardRow(rowButtons...))
		}

		if len(keyboardRows) > 0 {
			keyboard := tgbotapi.NewInlineKeyboardMarkup(keyboardRows...)
			msg.ReplyMarkup = keyboard
		}
	}

	sentMsg, err := b.api.Send(msg)
	if err != nil {
		return err
	}

	userSession.LastQuestionMessageID = sentMsg.MessageID

	return err
}

// handleAnswer processes the user's text reply, deleting messages to keep the chat clean.
func (b *Bot) handleAnswer(message *tgbotapi.Message, userSessions map[int64]*session.UserSession) error {
	chatID := message.Chat.ID
	answer := message.Text
	userReplyMessageID := message.MessageID

	// Always delete the user's incoming message to keep the chat clean.
	// We use defer to ensure it runs even if there's an error.
	defer func() {
		deleteUserMsg := tgbotapi.NewDeleteMessage(chatID, userReplyMessageID)
		_, _ = b.api.Request(deleteUserMsg)
	}()

	if answer == addOption {
		// If the user starts a new session, clean up the old question.
		if userSession, ok := userSessions[chatID]; ok && userSession.LastQuestionMessageID != 0 {
			deleteBotQuestion := tgbotapi.NewDeleteMessage(chatID, userSession.LastQuestionMessageID)
			_, _ = b.api.Request(deleteBotQuestion)
		}
		return b.startSession(chatID, userSessions)
	}

	userSession := userSessions[chatID]

	// Delegate validation to the session handler
	err := userSession.HandleAnswer(answer)
	if err != nil {
		// --- Handle Invalid Input ---
		log.Printf("Chat %d: Invalid user input. Error: %v", chatID, err)

		// The bot's original question is NOT deleted, providing context for the user's correction.
		// We send a new temporary message with the specific error.
		errorText := fmt.Sprintf("⚠️ %s\nPlease try again.", err.Error())
		errorMsg := tgbotapi.NewMessage(chatID, errorText)
		_, _ = b.api.Send(errorMsg) // Send the error and ignore the result for simplicity

		return err // Return original error to be logged
	}

	// --- Deletion Logic for VALID answers ---
	// The user's reply is already scheduled for deletion by the defer statement.
	// Now, delete the bot's previous question message.
	if userSession.LastQuestionMessageID != 0 {
		deleteBotQuestion := tgbotapi.NewDeleteMessage(chatID, userSession.LastQuestionMessageID)
		if _, err := b.api.Request(deleteBotQuestion); err != nil {
			log.Printf("Could not delete bot question message %d in chat %d: %v", userSession.LastQuestionMessageID, chatID, err)
		}
		// Reset it so we don't try to delete it again
		userSession.LastQuestionMessageID = 0
	}

	// --- Continue the conversation flow ---
	if userSession.IsSessionComplete() {
		return b.completeSession(chatID, userSession)
	}

	return b.askCurrentQuestion(chatID, userSessions)
}

// completeSession finishes the session.
func (b *Bot) completeSession(chatID int64, session *session.UserSession) error {
	if storage.UseDBToSave {
		// Save the responses to the database
		err := storage.SaveTransactionToDB(session.Answers)
		if err != nil {
			// Inform the user if saving failed
			errMsg := tgbotapi.NewMessage(chatID, "Sorry, there was an error saving your transaction. Please try again later.")
			_, sendErr := b.api.Send(errMsg)
			if sendErr != nil {
				log.Printf("Error sending save error message: %v", sendErr)
			}
			// Also return the original save error
			return fmt.Errorf("failed to save transaction to DB: %w", err)
		}
	} else {
		// Save the responses to file
		storage.SaveResponseToFile(session.Answers)
	}

	// Send a thank-you message and confirmation
	msg := tgbotapi.NewMessage(chatID,
		fmt.Sprintf("Thank you for your responses!\n\nHere are your answers:\nName: %s\nAmount: %f\nCurrency: %s\nDate: %s\nIs Claimable: %t\nPaid for Family: %t\nCategory: %s",
			session.Answers.Name, session.Answers.Amount, session.Answers.Currency, session.Answers.Date, session.Answers.IsClaimable, session.Answers.PaidForFamily, session.Answers.Category))

	_, err := b.api.Send(msg)
	if err != nil {
		log.Printf("Error sending confirmation message: %v", err)
		return err
	}

	err = b.sendDefaultMessage(chatID)
	if err != nil {
		return err
	}

	delete(userSessions, chatID)
	return nil
}

// handleCallbackQuery handles callback queries.
func (b *Bot) handleCallbackQuery(callbackQuery *tgbotapi.CallbackQuery, userSessions map[int64]*session.UserSession) error {
	chatID := callbackQuery.Message.Chat.ID
	messageID := callbackQuery.Message.MessageID // Get the ID of the message to delete

	// --- Delete the message with the inline keyboard ---
	// This gives the user immediate feedback that their action was received.
	deleteMsgConfig := tgbotapi.NewDeleteMessage(chatID, messageID)
	if _, err := b.api.Request(deleteMsgConfig); err != nil {
		// Log the error but don't stop execution.
		// The message might have already been deleted, or the bot might lack permissions.
		log.Printf("Could not delete message %d in chat %d: %v", messageID, chatID, err)
	}
	// --- End of deletion logic ---

	userSession, exists := userSessions[chatID]
	if !exists {
		// It's good practice to answer the callback even on error, to stop the loading animation.
		cb := tgbotapi.NewCallback(callbackQuery.ID, "Session expired. Please /add again.")
		_, _ = b.api.Request(cb) // Best-effort request
		return fmt.Errorf("session not found for chat ID: %d", chatID)
	}

	// Answer the callback query to remove the "loading" state from the button.
	// Since we are deleting the message, this is less critical, but still good practice
	// in case the deletion fails for some reason.
	callback := tgbotapi.NewCallback(callbackQuery.ID, "")
	if _, err := b.api.Request(callback); err != nil {
		log.Printf("Could not answer callback query %s: %v", callbackQuery.ID, err)
	}

	// The rest of your existing logic for handling the callback data follows.
	// I've included a refactored version below that is much cleaner.
	switch callbackQuery.Data {
	case "yes":
		if userSession.CurrentQuestion == session.QuestionIsClaimable {
			userSession.Answers.IsClaimable = true
		} else if userSession.CurrentQuestion == session.QuestionPaidForFamily {
			userSession.Answers.PaidForFamily = true
		}
	case "no":
		if userSession.CurrentQuestion == session.QuestionIsClaimable {
			userSession.Answers.IsClaimable = false
		} else if userSession.CurrentQuestion == session.QuestionPaidForFamily {
			userSession.Answers.PaidForFamily = false
		}
	default:
		if userSession.CurrentQuestion == session.QuestionName {
			userSession.Answers.Name = callbackQuery.Data
		} else if userSession.CurrentQuestion == session.QuestionCurrency {
			userSession.Answers.Currency = callbackQuery.Data
		} else if userSession.CurrentQuestion == session.QuestionCategory {
			userSession.Answers.Category = callbackQuery.Data
		}
	}

	preFilledExpense := session.CheckPreFilledExpense(userSession.Answers.Name, b.expense)

	prePaidForFamilyValue, isPrefilledValue := session.DefaultPaidForFamilyV2(userSession.Answers.Name, preFilledExpense)
	if userSession.CurrentQuestion == session.QuestionIsClaimable && isPrefilledValue {
		userSession.Answers.PaidForFamily = prePaidForFamilyValue
		userSession.CurrentQuestion++
	}

	if userSession.CurrentQuestion == session.QuestionPaidForFamily && len(session.DefaultCategoryV2(userSession.Answers.Name, preFilledExpense)) > 0 {
		userSession.Answers.Category = session.DefaultCategoryV2(userSession.Answers.Name, preFilledExpense)
		userSession.CurrentQuestion++
	}

	if userSession.CurrentQuestion == session.QuestionName && len(session.DefaultCurrencyV2(userSession.Answers.Name, preFilledExpense)) > 0 {
		userSession.Answers.Currency = session.DefaultCurrencyV2(userSession.Answers.Name, preFilledExpense)
		userSession.CurrentQuestion++
	}

	if userSession.Answers.Currency == preFilledExpense.Currency {
		userSession.CurrentQuestion++
	}

	userSession.CurrentQuestion++

	if userSession.IsSessionComplete() {
		return b.completeSession(chatID, userSession)
	}

	return b.askCurrentQuestion(chatID, userSessions)
}

func (b *Bot) sendDefaultMessage(chatID int64) error {
	messageText := fmt.Sprintf("Send %v to add new transaction or %v to view summary!", addOption, transactionsSummaryOption)
	_, err := b.api.Send(tgbotapi.NewMessage(chatID, messageText))

	return err
}
