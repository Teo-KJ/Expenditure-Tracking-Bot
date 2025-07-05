package config

// Top-level config struct
type Config struct {
	Database         DatabaseConfig    `yaml:"database"`
	TelegramConfig   TelegramConfig    `yaml:"telegram"`
	FrequentExpenses []FrequentExpense `yaml:"frequent_expenses"`
}

/*func GetConfig() Config {
	return Config{}
}*/
