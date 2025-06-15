// Package telegram предоставляет функциональность для отправки уведомлений в Telegram
package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TelegramMessage представляет сообщение для отправки в Telegram
type TelegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

// TelegramClient клиент для работы с Telegram Bot API
type TelegramClient struct {
	botToken string
	chatID   string
	httpClient *http.Client
}

// NewTelegramClient создает новый клиент для работы с Telegram
func NewTelegramClient(botToken, chatID string) *TelegramClient {
	return &TelegramClient{
		botToken: botToken,
		chatID:   chatID,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendMessage отправляет сообщение в Telegram
func (c *TelegramClient) SendMessage(text string) error {
	if c.botToken == "" || c.chatID == "" {
		return fmt.Errorf("telegram bot token or chat ID not configured")
	}

	message := TelegramMessage{
		ChatID:    c.chatID,
		Text:      text,
		ParseMode: "HTML",
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal telegram message: %v", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.botToken)
	
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status code: %d", resp.StatusCode)
	}

	return nil
}

// SendBusinessRegistrationNotification отправляет уведомление о новой заявке на регистрацию бизнеса
func (c *TelegramClient) SendBusinessRegistrationNotification(serviceName, contactName, contactPhone, city string) error {
	message := fmt.Sprintf(
		"🆕 <b>Новая заявка на регистрацию сервисного центра</b>\n\n"+
		"📱 <b>Название:</b> %s\n"+
		"👤 <b>Контактное лицо:</b> %s\n"+
		"📞 <b>Телефон:</b> %s\n"+
		"🏙 <b>Город:</b> %s\n\n"+
		"⏰ <i>%s</i>",
		serviceName,
		contactName,
		contactPhone,
		city,
		time.Now().Format("02.01.2006 15:04:05"),
	)

	return c.SendMessage(message)
}