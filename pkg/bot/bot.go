package bot

import (
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"main/pkg/session"
	"main/pkg/storage"
)

// Map to track ongoing sessions (active users)
var userSessions = make(map[int64]*session.UserSession)

// Bot represents the Telegram bot.
type Bot struct {
	api *tgbotapi.BotAPI
}

// NewBot creates a new bot instance.
func NewBot(token string) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot API: %w", err)
	}
	api.Debug = true
	log.Printf("Authorized on account %s", api.Self.UserName)
	return &Bot{api: api}, nil
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

	if message.Text == "/add" {
		return b.startSession(chatID, userSessions)
	}

	if userSession, exists := userSessions[chatID]; exists {
		return b.handleAnswer(chatID, userSession, message.Text)
	}

	msg := tgbotapi.NewMessage(chatID, "Send /add to begin!")
	_, err := b.api.Send(msg)
	return err
}

// startSession starts a new session for the user.
func (b *Bot) startSession(chatID int64, userSessions map[int64]*session.UserSession) error {
	userSessions[chatID] = session.NewUserSession()
	return b.askCurrentQuestion(chatID, userSessions)
}

// askCurrentQuestion sends the current question to the user.
func (b *Bot) askCurrentQuestion(chatID int64, userSessions map[int64]*session.UserSession) error {
	userSession := userSessions[chatID]
	question := session.Questions[userSession.CurrentQuestion]

	msg := tgbotapi.NewMessage(chatID, question)

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

	_, err := b.api.Send(msg)
	return err
}

// handleAnswer processes the user's answer.
func (b *Bot) handleAnswer(chatID int64, session *session.UserSession, answer string) error {
	err := session.HandleAnswer(answer)
	if err != nil {
		return err
	}

	if session.IsSessionComplete() {
		return b.completeSession(chatID, session)
	}

	return b.askCurrentQuestion(chatID, userSessions)
}

// completeSession finishes the session.
func (b *Bot) completeSession(chatID int64, session *session.UserSession) error {
	storage.SaveResponseToFile(session.Answers)

	msg := tgbotapi.NewMessage(chatID,
		fmt.Sprintf("Thank you for your responses!\n\nHere are your answers:\nName: %s\nAmount: %f\nCurrency: %s\nDate: %s\nIs Claimable: %t\nPaid for Family: %t",
			session.Answers.Name, session.Answers.Amount, session.Answers.Currency, session.Answers.Date, session.Answers.IsClaimable, session.Answers.PaidForFamily))
	_, err := b.api.Send(msg)
	if err != nil {
		return err
	}

	delete(userSessions, chatID)
	return nil
}

// handleCallbackQuery handles callback queries.
func (b *Bot) handleCallbackQuery(callbackQuery *tgbotapi.CallbackQuery, userSessions map[int64]*session.UserSession) error {
	chatID := callbackQuery.Message.Chat.ID
	userSession, exists := userSessions[chatID]
	if !exists {
		return fmt.Errorf("session not found for chat ID: %d", chatID)
	}

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
		if userSession.CurrentQuestion == session.QuestionCurrency {
			userSession.Answers.Currency = callbackQuery.Data
		}
	}

	userSession.CurrentQuestion++

	if userSession.IsSessionComplete() {
		err := b.completeSession(chatID, userSession)
		if err != nil {
			return err
		}
		return nil
	}

	err := b.askCurrentQuestion(chatID, userSessions)
	if err != nil {
		return err
	}

	callback := tgbotapi.NewCallback(callbackQuery.ID, "")
	if _, err := b.api.Request(callback); err != nil {
		return err
	}

	return nil
}
