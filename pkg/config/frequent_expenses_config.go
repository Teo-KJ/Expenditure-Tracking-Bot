package config

type FrequentExpense struct {
	Name          string `yaml:"name"`
	Category      string `yaml:"category"`
	Currency      string `yaml:"currency"`
	IsClaimable   bool   `yaml:"is_claimable"`
	PaidForFamily bool   `yaml:"paid_for_family"`
}
