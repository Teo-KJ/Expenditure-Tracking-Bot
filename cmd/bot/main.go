package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"main/pkg/bot"
	"main/pkg/config"
	"main/pkg/session"
	"main/pkg/storage"
	"net/http"
	"os"
	"os/exec"
	"time"
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

	shouldUseML := false // todo: remove boolean variable and switch to configs
	if shouldUseML {
		startPythonService()

		label, score, err := classifyText("Grab to airport")
		if err != nil {
			log.Fatalf("Prediction failed: %v", err)
		}

		fmt.Printf("Predicted label: %s (%.2f%% confidence)\n", label, score*100)
	}

	myBot, err := bot.NewBot(cfg.TelegramConfig.Token)
	if err != nil {
		log.Panic(err)
	}

	userSessions := make(map[int64]*session.UserSession)
	myBot.StartListening(userSessions)
}

func startPythonService() {
	cmd := exec.Command("uvicorn", "text_classifier.app:app", "--host", "0.0.0.0", "--port", "8000")

	// Optional: redirect Python stdout/stderr to Go console
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Start the process (non-blocking)
	err := cmd.Start()
	if err != nil {
		log.Fatalf("Failed to start Python service: %v", err)
	}

	// Give the Python service time to boot
	fmt.Println("Starting Python service...")
	time.Sleep(3 * time.Second)
}

func classifyText(text string) (string, float64, error) {
	reqBody := map[string]string{"text": text}
	jsonData, _ := json.Marshal(reqBody)

	resp, err := http.Post("http://localhost:8000/predict", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", 0, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			panic(err)
		}
	}(resp.Body)

	body, _ := io.ReadAll(resp.Body)

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", 0, err
	}

	label := result["label"].(string)
	score := result["score"].(float64)
	return label, score, nil
}
