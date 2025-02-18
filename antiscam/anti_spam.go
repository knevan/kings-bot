package antiscam

import (
	"fmt"
	"log"
	"regexp"

	"github.com/bwmarrin/discordgo"
)

var (
	// Regex pattern for reducing false positive
	spamRegexPattern = []string{
		`(?i)\b(?:free|get|claim|gifts?)(?:\s*\+\s*|\s*-\s*|\s*)?(?:steam|gifts?|keys?|cards?)(?:\s*\+\s*|\s*-\s*|\s*)?(giveaway)\b`,
		`(?i)\b(?:free|get|claim|gifts?)(?:\s*\+\s*|\s*-\s*|\s*)?(?:steam|gifts?|keys?|cards?)\b`,
		`(?i)\b(?:gift|steam)(?:\s*\+\s*|\s*-\s*|\s*)?(?:cards?|\$50|50\$)\b`,
		`(?i)\b(?:free|best|onlyfans|teen|NSFW|sex|leaks?)(?:\s*\+\s*|\s*-\s*|\s*)?(?:porn|NSFW|hub|onlyfans|teen|sex|leaks?)\b`,
		`(?i)\b(?:free|hot|nudes?|hentai)(?:\s*\+\s*|\s*-\s*|\s*)?(?:porn|pussys?|nudes?)\b`,
		`(?i)\b(?:stake|airdrop|claim|rewards?)(?:\s*\+\s*|\s*-\s*|\s*)?(?:stake|airdrop|claim|rewards?)\b`,
		`(?i)\b(?:nitro)(?:\s*\+\s*|\s*-\s*|\s*)?(?:giveaway)`,
		`(?i)\b(?:crypto|casino|fasts?)(?:\s*\+\s*|\s*-\s*|\s*)?(?:giveaway|payouts?|luck|catch)\b`,
	}
	// Slice to store regex pattern
	compiledRegex []*regexp.Regexp
)

// Init Precompile regex pattern during initialization
func Init() {
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

			/*_, err := s.ChannelMessageSendReply(m.ChannelID, responseSpam, m.Reference())
			if err != nil {
				fmt.Println("Failed to send delete message", err)
			}*/

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

			// Banned spam user from server
			err = s.GuildBanCreateWithReason(m.GuildID, m.Author.ID, "spamming detected", 0)
			if err != nil {
				fmt.Printf("Error when banning user %s: %v\n\n", userName, err)
			} else {
				fmt.Printf("User %s has beed banned for spamming\n", userName)

			}
			return
		}
	}
}
