package automod

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"kings-bot/db"
)

// RapidMessageData struct to store history messages per-user.
type RapidMessageData struct {
	MessageContent string
	// ChannelID      string
	Timestamp time.Time
}

// UserMessageRecord struct to store history messages for all user.
type UserMessageRecord struct {
	messages     map[string][]RapidMessageData
	messageMutex sync.Mutex
}

var userMessages = UserMessageRecord{
	messages: make(map[string][]RapidMessageData),
}

// CheckRapidMessages function to check for rapid message spamming more than 3 times.
func CheckRapidMessages(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot || m.GuildID == "" {
		return
	}

	// Get userID and message content.
	userID := m.Author.ID
	content := m.Content
	guildID := m.GuildID
	banDuration := 5 * time.Minute
	banEndTIme := time.Now().Add(banDuration)
	banUnixTime := banEndTIme.Unix()
	reason := content

	// Lock the mutex to ensure thread safety during message addition.
	userMessages.messageMutex.Lock()
	defer userMessages.messageMutex.Unlock()

	// Initialize the map for the specific user if it doesn't exist.
	if _, exists := userMessages.messages[userID]; !exists {
		userMessages.messages[userID] = []RapidMessageData{}
	}

	// Add new message to the map for the specific user.
	userMessages.messages[userID] = append(userMessages.messages[userID], RapidMessageData{
		MessageContent: content,
		Timestamp:      time.Now(),
	})

	// Clean up old messages older than 1 minute in the map for a specific user.
	cleanOldMessages(userID)

	// Count identical message in 1 minute
	count := countIdenticalMessage(userID, content)

	// Check if identical message have been sent 3 times or more
	if count >= 3 {
		err := s.GuildBanCreateWithReason(m.GuildID, userID, reason, 7)
		if err != nil {
			log.Printf("Failed to ban user %s: %v", userID, err)
			return
		}

		// Send ban confirmation message
		sendBanConfirmation(s, m, reason)

		// Send direct message to the banned user
		sendDirectMessage(s, m, reason, banUnixTime)

		// Add temporary ban to the database
		err = db.AddTempBan(userID, guildID, s.State.User.ID, banDuration, reason)
		if err != nil {
			log.Printf("Error adding temporary ban to database: %v", err)
		}

		// Send ban log message
		sendBanLogMessage(s, m, reason)

		// Remove user history message after ban
		delete(userMessages.messages, userID)
	}
}

// Clean up old messages older than 1 minute in the map for a specific user.
func cleanOldMessages(userID string) {
	cutoffContent := time.Now().Add(-1 * time.Minute)
	var updateMessages []RapidMessageData

	for _, msg := range userMessages.messages[userID] {
		if msg.Timestamp.After(cutoffContent) {
			updateMessages = append(updateMessages, msg)
		}
	}

	// Update the map with the updated slice of messages.
	userMessages.messages[userID] = updateMessages
}

// Count identical message in 1 minute
func countIdenticalMessage(userID, content string) int {
	count := 0
	for _, msg := range userMessages.messages[userID] {
		if msg.MessageContent == content {
			count++
		}
	}
	return count
}

func sendBanConfirmation(s *discordgo.Session, m *discordgo.MessageCreate, reason string) {
	embedConfirmation := &discordgo.MessageEmbed{
		Title: "Spam Rapid Message Detected",
		Color: 0xff0000,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Banned User",
				Value:  fmt.Sprintf("%s (Username: %s)", m.Author.Mention(), m.Author.Username),
				Inline: true,
			},
			{
				Name:   "Reason",
				Value:  fmt.Sprintf("```%s```", reason),
				Inline: false,
			},
		},
	}

	msgEmbedSend := &discordgo.MessageSend{
		Embed: embedConfirmation,
		AllowedMentions: &discordgo.MessageAllowedMentions{
			Parse: []discordgo.AllowedMentionType{},
		},
		Reference: m.Reference(),
	}

	_, err := s.ChannelMessageSendComplex(m.ChannelID, msgEmbedSend)
	if err != nil {
		log.Print("Failed to send ban confirmation message", err)
	}
}
