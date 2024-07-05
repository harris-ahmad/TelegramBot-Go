package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
	"github.com/valyala/fasthttp"

	"TelegramBot/config"
	"TelegramBot/models"
)

func main() {
	config.ConnectSQL()
	defer config.DB.Close()
	config.DB.AutoMigrate(&models.User{}, &models.Alert{})

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_TOKEN"))
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	c := cron.New()
	c.AddFunc("0 8 * * *", func() { dailySummary(bot) })
	c.AddFunc("*/5 * * * *", func() { checkAlerts(bot) })
	c.Start()

	for update := range updates {
		if update.Message != nil {
			handleUpdate(bot, update.Message)
		}
	}
}

func handleUpdate(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	switch {
	case strings.HasPrefix(message.Text, "/start"):
		handleStart(bot, message)
	case strings.HasPrefix(message.Text, "/help"):
		handleHelp(bot, message)
	case strings.HasPrefix(message.Text, "/register"):
		handleRegister(bot, message)
	case strings.HasPrefix(message.Text, "/get_token"):
		handleGetToken(bot, message)
	case strings.HasPrefix(message.Text, "/set_alert"):
		handleSetAlert(bot, message)
	case strings.HasPrefix(message.Text, "/list_alerts"):
		handleListAlerts(bot, message)
	case strings.HasPrefix(message.Text, "/remove_alert"):
		handleRemoveAlert(bot, message)
	}
}

func handleStart(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	msg := tgbotapi.NewMessage(message.Chat.ID, "Welcome to Crypto Data Bot! Use /register to get started.")
	bot.Send(msg)
}

func handleHelp(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	response := `Commands:
	/register - Register with the bot
	/get_token [token symbol] - Get cryptocurrency data
	/set_alert [token symbol] [price threshold] [above/below] - Set a price alert
	/list_alerts - List all alerts
	/remove_alert [alert ID] - Remove an alert`
	msg := tgbotapi.NewMessage(message.Chat.ID, response)
	bot.Send(msg)
}

func handleRegister(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	userId := strconv.FormatInt(message.From.ID, 10)
	username := message.From.UserName
	if username == "" {
		username = message.From.FirstName
	}

	var user models.User
	config.DB.Where("user_id = ?", userId).First(&user)
	if user.ID == 0 {
		config.DB.Create(&models.User{UserId: userId, Username: username})
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "You are now registered!"))
	} else {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "You are already registered."))
	}
}

func handleGetToken(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	tokenSymbol := strings.Split(message.Text, " ")[1]
	data, err := getCryptoData(tokenSymbol)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Failed to fetch data. Please check the token symbol and try again."))
		return
	}

	response := fmt.Sprintf(
		"Symbol: %s\nCurrent Price: $%.2f\nMarket Cap: $%d\nNumber of Holders: %d\n24h Trading Volume: $%d\nPrice Change (24h): %.2f%%",
		strings.ToUpper(data.Symbol), data.CurrentPrice, data.MarketCap, data.CirculatingSupply, data.TotalVolume, data.PriceChange24h,
	)
	bot.Send(tgbotapi.NewMessage(message.Chat.ID, response))
}

func handleSetAlert(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	parts := strings.Split(message.Text, " ")
	tokenSymbol := parts[1]
	priceThreshold, _ := strconv.ParseFloat(parts[2], 64)
	condition := parts[3]

	userId := strconv.FormatInt(message.From.ID, 10)
	var user models.User
	config.DB.Where("user_id = ?", userId).First(&user)
	if user.ID == 0 {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "You need to register first using /register."))
		return
	}

	alert := models.Alert{UserId: userId, TokenSymbol: tokenSymbol, PriceThreshold: priceThreshold, Condition: condition}
	config.DB.Create(&alert)
	bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Alert set successfully!"))
}

func handleListAlerts(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	userId := strconv.FormatInt(message.From.ID, 10)
	var alerts []models.Alert
	config.DB.Where("user_id = ?", userId).Find(&alerts)

	if len(alerts) == 0 {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "No active alerts found."))
		return
	}

	response := "Your active alerts:\n"
	for i, alert := range alerts {
		response += fmt.Sprintf("%d. [%d] %s - %s $%.2f\n", i+1, alert.ID, strings.ToUpper(alert.TokenSymbol), alert.Condition, alert.PriceThreshold)
	}
	bot.Send(tgbotapi.NewMessage(message.Chat.ID, response))
}

func handleRemoveAlert(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	alertId := strings.Split(message.Text, " ")[1]
	var alert models.Alert
	config.DB.First(&alert, alertId)
	if alert.ID == 0 {
		bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Alert not found. Please check the alert ID."))
		return
	}

	config.DB.Delete(&alert)
	bot.Send(tgbotapi.NewMessage(message.Chat.ID, "Alert removed successfully!"))
}

func dailySummary(bot *tgbotapi.BotAPI) {
	var users []models.User
	config.DB.Find(&users)

	for _, user := range users {
		var alerts []models.Alert
		config.DB.Where("user_id = ?", user.UserId).Find(&alerts)

		if len(alerts) > 0 {
			response := "Daily summary of your active alerts:\n"
			for i, alert := range alerts {
				response += fmt.Sprintf("%d. [%d] %s - %s $%.2f\n", i+1, alert.ID, strings.ToUpper(alert.TokenSymbol), alert.Condition, alert.PriceThreshold)
			}
			userIdInt, _ := strconv.ParseInt(user.UserId, 10, 64)
			bot.Send(tgbotapi.NewMessage(userIdInt, response))
		}
	}
}

func checkAlerts(bot *tgbotapi.BotAPI) {
	var alerts []models.Alert
	config.DB.Find(&alerts)

	for _, alert := range alerts {
		data, err := getCryptoData(alert.TokenSymbol)
		if err != nil {
			log.Println("Error fetching crypto data:", err)
			continue
		}

		var alertTriggered bool
		if (alert.Condition == "above" && data.CurrentPrice > alert.PriceThreshold) || (alert.Condition == "below" && data.CurrentPrice < alert.PriceThreshold) {
			alertTriggered = true
		}

		if alertTriggered {
			message := fmt.Sprintf("Alert triggered: %s is %s $%.2f. Current price: $%.2f", strings.ToUpper(alert.TokenSymbol), alert.Condition, alert.PriceThreshold, data.CurrentPrice)
			userIdInt, _ := strconv.ParseInt(alert.UserId, 10, 64)
			bot.Send(tgbotapi.NewMessage(userIdInt, message))
		}
	}
}

func getCryptoData(tokenSymbol string) (CryptoData, error) {
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/coins/markets?vs_currency=usd&ids=%s", tokenSymbol)
	status, body, err := fasthttp.Get(nil, url)
	if err != nil {
		return CryptoData{}, err
	}

	if status != fasthttp.StatusOK {
		return CryptoData{}, fmt.Errorf("unexpected status code: %d", status)
	}

	var data []CryptoData
	err = json.Unmarshal(body, &data)
	if err != nil {
		return CryptoData{}, err
	}

	if len(data) == 0 {
		return CryptoData{}, fmt.Errorf("no data found")
	}

	return data[0], nil
}

type CryptoData struct {
	Symbol            string  `json:"symbol"`
	CurrentPrice      float64 `json:"current_price"`
	MarketCap         int64   `json:"market_cap"`
	CirculatingSupply int64   `json:"circulating_supply"`
	TotalVolume       int64   `json:"total_volume"`
	PriceChange24h    float64 `json:"price_change_percentage_24h"`
}
