package main

import (
	"gopkg.in/yaml.v3"
	"log"
	"main/pkg/bot"
	"main/pkg/config"
	"main/pkg/session"
	"main/pkg/storage"
	"os"
)

func main() {
	// Read the YAML file content
	yamlFile, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("Error reading YAML file: %v", err)
	}

	// Create a variable of your config struct type
	var cfg config.Config

	// Unmarshal the YAML data into the struct
	err = yaml.Unmarshal(yamlFile, &cfg)
	if err != nil {
		log.Fatalf("Error unmarshalling YAML: %v", err)
	}

	if storage.UseDBToSave {
		err = storage.InitDB(cfg.Database)
		if err != nil {
			log.Panic(err)
		}
		defer storage.CloseDB()
	}

	myBot, err := bot.NewBot(cfg.TelegramConfig.Token, cfg.FrequentExpenses)
	if err != nil {
		log.Panic(err)
	}

	userSessions := make(map[int64]*session.UserSession)
	myBot.StartListening(userSessions)
}
