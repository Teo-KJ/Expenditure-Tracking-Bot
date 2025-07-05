package session

import "main/pkg/config"

func CheckPreFilledExpense(transactionName string, preFilledExpenses []config.FrequentExpense) *config.FrequentExpense {
	for i := 0; i < len(preFilledExpenses); i++ {
		if preFilledExpenses[i].Name == transactionName {
			return &preFilledExpenses[i]
		}
	}

	return nil
}

func DefaultCategory(transactionName string) string {
	switch transactionName {
	case DailyTransportExpenses:
		return TransportCategory
	case DinnerForTheFamily, GroceriesFromPandamart:
		return FoodCategory
	case MonthlyGymMembership:
		return HealthAndFitnessCategory
	case GOMOMobilePlan, AppleICloudSubscription, SpotifyMonthlySubscription, GoogleOneSubscription:
		return EntertainmentCategory
	default:
		return ""
	}
}

func DefaultCategoryV2(transactionName string, preFilledExpense *config.FrequentExpense) string {
	if preFilledExpense == nil {
		return ""
	}

	if preFilledExpense.Name == transactionName {
		return preFilledExpense.Category
	}

	return ""
}

func DefaultPaidForFamily(transactionName string) (bool, bool) {
	switch transactionName {
	case DinnerForTheFamily, GroceriesFromPandamart:
		return true, true
	case MonthlyGymMembership, GOMOMobilePlan, AppleICloudSubscription, SpotifyMonthlySubscription, GoogleOneSubscription:
		return false, true
	default:
		return false, false
	}
}

func DefaultPaidForFamilyV2(transactionName string, preFilledExpense *config.FrequentExpense) (bool, bool) {
	if preFilledExpense == nil {
		return false, false
	}

	if preFilledExpense.Name == transactionName {
		return preFilledExpense.PaidForFamily, true
	}

	return false, false
}

func DefaultCurrency(transactionName string) string {
	switch transactionName {
	case GroceriesFromPandamart, MonthlyGymMembership, GOMOMobilePlan, AppleICloudSubscription, GoogleOneSubscription:
		return SGDCurrency
	default:
		return ""
	}
}

func DefaultCurrencyV2(transactionName string, preFilledExpense *config.FrequentExpense) string {
	if preFilledExpense == nil {
		return ""
	}

	if preFilledExpense.Name == transactionName {
		return preFilledExpense.Currency
	}

	return ""
}
