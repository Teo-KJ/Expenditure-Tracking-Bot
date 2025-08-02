package session

const (
	QuestionName = iota
	QuestionAmount
	QuestionCurrency
	QuestionDate
	QuestionIsClaimable
	QuestionPaidForFamily
	QuestionCategory
	QuestionCount // Should be last; represents the total number of questions
)

// Questions array for the process
var Questions = []string{
	"What is the name of the transaction?",
	"How much is the transaction?",
	"What currency is the transaction in?",
	"What is the date of transaction? \\(i\\.e\\., DD\\.MM\\.YY\\)", // Added format hint
	"Is it claimable? \\(yes/no\\)",                                 // Added format hint
	"Is it paid for the family? \\(yes/no\\)",                       // Added format hint
	"What is the category of transaction?",
}
