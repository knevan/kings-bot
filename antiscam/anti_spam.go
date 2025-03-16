package antiscam

import (
	"fmt"
	"log"
	"regexp"
	"time"

	"github.com/bwmarrin/discordgo"

	"kings-bot/db"
)

var (
	banLogChannelID string

	// Regex pattern for reducing false positive
	spamRegexPattern = []string{
		// reminder [\W\s]*: Matches zero or more non-word characters or spaces.
		`(?i)\b(?:free|get|claim|steam|gifts?|giftaways?|gift away)\b.*?[\s\+\-@\$]*?(?:steam|gifts?|keys?|cards?|giftaways?|gift away)\b`,
		`(?i)\b(?:free|best|onlyfans|teen|NSFW|hub|sex|leaks?|hot|nudes?|hentai)\b,*?[\W*?](?:porn|best|NSFW|hub|hot|onlyfans|teen|sex|leaks?|pussys?|nudes?|hentai)\b`,
		`(?i)\b(?:stake|airdrop|claim|rewards?)\b.*?[\s\+\-@\$]*?(?:stake|airdrop|claim|rewards?)\b`,
		`(?i)\b(?:nitro|free|giveaways?|give aways?)\b.*?[\s\+\-@\$]*?(?:nitro|free|giveaways?|give aways?)\b`,
		`(?i)\b(?:crypto|casino|fasts?)\b.*?[\s\+\-@\$]*?(?:giveaways?|payouts?|luck|catch)\b`,
		`(?i)\b(?:from|steam|free|gifts?)\b\s*[\W\s]*(?:-?\s*\d+\s*\$?|\$?\s*-?\s*\d+)|\b(?:-?\s*\d+\s*\$?|\$?\s*-?\s*\d+)\s*[\W\s]*\b(?:from|steam|free|gifts?)\b`,
	}
	// Slice to store regex pattern
	compiledRegex []*regexp.Regexp
)

// Init Precompile regex pattern during initialization
func Init(logChannelId string) {
	banLogChannelID = logChannelId

	compiledRegex = make([]*regexp.Regexp, len(spamRegexPattern))
	for i, pattern := range spamRegexPattern {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			log.Fatalf("Failed to compile regex pattern: %s, error: %v", pattern, err)
		}
		compiledRegex[i] = compiled
	}
}

func DeleteSpamMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	fmt.Println("Message Recieved:", m.Content)

	// Loop regex pattern
	for _, regex := range compiledRegex {
		if regex.MatchString(m.Content) {
			log.Printf("Spam detected in message from %s", m.Author.Username)

			// Find the specific spam message
			// matchedWord := regex.FindString(m.Content)
			matchedWord := m.Content

			// Reply and Response spam chat with reason why
			responseSpam := fmt.Sprintf("**%s**", matchedWord)

			// Embed message for simplicity and better view
			embed := &discordgo.MessageEmbed{
				Title: "Spam Message Detected",
				// Description: responseSpam,
				Color: 0xff0000,
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:   "Banned User",
						Value:  fmt.Sprintf("%s (Username: %s)", m.Author.Mention(), m.Author.Username),
						Inline: true,
					},
					{
						Name:   "Reason",
						Value:  responseSpam,
						Inline: false,
					},
				},
			}

			msgSend := &discordgo.MessageSend{
				Embed: embed,
				AllowedMentions: &discordgo.MessageAllowedMentions{
					Roles: []string{},
					Users: []string{},
					Parse: []discordgo.AllowedMentionType{},
				},
				Reference: m.Reference(),
			}

			_, err := s.ChannelMessageSendComplex(m.ChannelID, msgSend)
			if err != nil {
				fmt.Println("Failed to send delete message", err)
			}

			// Delete spam chat from channel
			err = s.ChannelMessageDelete(m.ChannelID, m.ID)
			if err != nil {
				fmt.Println("Error when trying to delete message", err)
			}

			// Banned spam chats members
			guildID := m.GuildID
			// userID := m.Author.ID
			userName := m.Author.Username

			if guildID == "" {
				fmt.Println("Guild ID cant be found, can't ban member")
				return
			}

			// Calculate the ban end time based on Unix timestamp
			banDuration := 3 * time.Minute
			banEndTime := time.Now().Add(banDuration)
			banUnixTime := banEndTime.Unix()

			// Create a single-use, never-expiring discord invite link
			invite, err := s.ChannelInviteCreate(m.ChannelID, discordgo.Invite{
				MaxAge:    0,
				MaxUses:   1,
				Temporary: false,
			})
			if err != nil {
				fmt.Printf("Error creating invite: %v", err)
			}

			// Create Embed Message for Direct Message
			dmEmbed := &discordgo.MessageEmbed{
				Title: "You have been banned from the KinG Server",
				Description: fmt.Sprintf("You have been banned until <t:%d:F> <t:%d:R> due to Spamming and Compromissed Account. \n \n"+
					"If you have gained access and secured your account, you can rejoin after the ban period using this one time invite link:", banUnixTime, banUnixTime),
				Color: 0xff0000,
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:  "Reason",
						Value: responseSpam,
					},
				},
			}

			// Send DM to banned user
			dmChannel, err := s.UserChannelCreate(m.Author.ID)
			if err != nil {
				log.Printf("Failed to send DM to banned user: %v", err)
			} else if dmChannel == nil {
				log.Printf("dmChannel is nil unexpectedly")
			} else {
				// Send DM Embed Message
				_, err = s.ChannelMessageSendEmbed(dmChannel.ID, dmEmbed)
				if err != nil {
					fmt.Printf("Failed to send DM: %v", err)
					return
				}

				// Send DM invite link
				inviteMessage := fmt.Sprintf("https://discord.gg/%s", invite.Code)
				_, err = s.ChannelMessageSend(dmChannel.ID, inviteMessage)
				if err != nil {
					fmt.Printf("Failed to send invite link: %v", err)
					return
				}
			}

			// Banned spam user from server
			reasonBan := "Spamming detected"
			err = s.GuildBanCreateWithReason(m.GuildID, m.Author.ID, reasonBan, 7)
			if err != nil {
				fmt.Printf("Error when banning user %s: %v\n\n", userName, err)
			} else {
				fmt.Printf("User %s has beed banned for spamming\n", userName)

				// Add temporary ban to database
				err = db.AddTempBan(m.Author.ID, m.GuildID, s.State.User.ID, banDuration, reasonBan)
				if err != nil {
					log.Printf("Error adding temporary ban to database: %v", err)
				}

				// Send log message to ban-log channel
				logEmbed := &discordgo.MessageEmbed{
					Title: "User Banned",
					Color: 0xff0000,
					Fields: []*discordgo.MessageEmbedField{
						{
							Name:   "Username",
							Value:  m.Author.Username,
							Inline: true,
						},
						{
							Name:   "User ID",
							Value:  m.Author.ID,
							Inline: true,
						},
						{
							Name:   "Reason",
							Value:  responseSpam,
							Inline: false,
						},
					},
					Timestamp: time.Now().Format(time.RFC3339),
				}

				_, err := s.ChannelMessageSendEmbed(banLogChannelID, logEmbed)
				if err != nil {
					fmt.Printf("Failed to send log message: %v\n", err)
				}
			}
			return
		}
	}
}
