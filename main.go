package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var (
	Token string
	// YoutubeChannelID YoutubeAPIKey    string
	YoutubeChannelID string
	DiscordChannelID string
	VerifyToken      string
	Port             = "8080"
	// checkInterval           = 20 * time.Second
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

// YoutubeNotification struct for xml payload from YouTube
type YoutubeNotification struct {
	XMLName xml.Name `xml:"feed"`
	Entry   struct {
		VideoID   string `xml:"videoID"`
		ChannelID string `xml:"channelID"`
		Title     string `xml:"title"`
		Status    struct {
			Type string `xml:"type,attr"`
		} `xml:"status"`
	} `xml:"entry"`
}

// Precompile regex pattern during initialization
func init() {
	compiledRegex = make([]*regexp.Regexp, len(spamRegexPattern))
	for i, pattern := range spamRegexPattern {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			log.Fatalf("Failed to compile regex pattern: %s, error: %v", pattern, err)
		}
		compiledRegex[i] = compiled
	}
}

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file, using environment variables directly")
	}

	// Get Token from .env file
	Token = os.Getenv("DISCORD_BOT_TOKEN")
	// YoutubeAPIKey = os.Getenv("YOUTUBE_API_KEY")
	YoutubeChannelID = os.Getenv("YOUTUBE_CHANNEL_ID")
	DiscordChannelID = os.Getenv("DISCORD_CHANNEL_ID")
	VerifyToken = os.Getenv("VERIFY_TOKEN")

	// Discord bot Session
	session, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("Error Discord Session", err)
		return
	}

	// Handler for Spam Message
	session.AddHandler(deleteSpamMessage)

	// Connecting bot with bot token
	err = session.Open()
	if err != nil {
		log.Println("Error when connecting", err)
		return
	}
	fmt.Println("Bot working")

	// Setup http server for YouTube Webhook
	http.HandleFunc("/youtube/webhook", func(w http.ResponseWriter, r *http.Request) {
		handleYoutubeWebhook(w, r, session)
	})

	// Subscribe youtube channel
	err = subscribeYoutubeChannel(YoutubeChannelID)
	if err != nil {
		log.Printf("Error subscribing channel: %v", err)
	}

	// goroutine for checkLiveStream
	go func() {
		log.Printf("Starting webhook server on %s", Port)
		if err := http.ListenAndServe(":"+Port, nil); err != nil {
			log.Fatal(err)
		}
	}()

	// Kill discord bot
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanup and Close bot session
	err = session.Close()
	if err != nil {
		log.Println("Cant kill discord")
	}
}

// Handle Webhook
func handleYoutubeWebhook(w http.ResponseWriter, r *http.Request, s *discordgo.Session) {
	log.Printf("Received webhook request: %s %s", r.Method, r.URL.Path)

	// Handle verification GET request
	if r.Method == "GET" {
		challenge := r.URL.Query().Get("hub.challenge")
		verifyToken := r.URL.Query().Get("hub.verify_token")

		log.Printf("Verification attempt - Token: %s, Challenge: %s", verifyToken, challenge)
		if verifyToken != VerifyToken {
			log.Printf("Invalid verify token received: %s", verifyToken)
			http.Error(w, "Invalid verification token.", http.StatusForbidden)
			return
		}

		log.Printf("Verification successful, responding with challenge: %s", challenge)

		if _, err := w.Write([]byte(challenge)); err != nil {
			log.Printf("Error writing challenge response: %v", err)
			http.Error(w, "Failed to write response", http.StatusInternalServerError)
			return
		}
	}

	// Handle notification POST request
	if r.Method == "POST" {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading body", http.StatusInternalServerError)
			return
		}
		defer func() {
			if err := r.Body.Close(); err != nil {
				log.Printf("Error closing body: %v", err)
			}
		}()

		var notification YoutubeNotification
		if err := xml.Unmarshal(body, &notification); err != nil {
			http.Error(w, "Error parsing XML youtube notification", http.StatusBadRequest)
		}

		// Check if channel start Livestream
		if notification.Entry.Status.Type == "Live" {
			message := fmt.Sprintf("\\n%s\\nhttps://youtube.com/watch?v=%s",
				notification.Entry.Title,
				notification.Entry.VideoID)

			_, err = s.ChannelMessageSend(DiscordChannelID, message)
			if err != nil {
				log.Printf("Error Sending Discord Message: %v", err)
			} else {
				log.Printf("%s", notification.Entry.Title)
			}
		}
		w.WriteHeader(http.StatusOK)
	}
}

func subscribeYoutubeChannel(channelID string) error {
	callbackURL := fmt.Sprintf("https://5506-180-252-118-216.ngrok-free.app/youtube/webhook")
	topicURL := fmt.Sprintf("https://www.youtube.com/xml/feeds/videos.xml?channel_id=%s", channelID)

	values := url.Values{}
	values.Set("hub.callback", callbackURL)
	values.Set("hub.topic", topicURL)
	values.Set("hub.verify_token", VerifyToken)
	values.Set("hub.mode", "subscribe")
	// values.Set("hub.lease_seconds", "432000")

	resp, err := http.PostForm("https://pubsubhubbub.appspot.com/subscribe", values)
	if err != nil {
		return fmt.Errorf("error subscribing: %v", err)
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("Error closing body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("something error in here: %d", resp.StatusCode)
	}
	log.Printf("Success subscribe to channel: %s", channelID)
	return nil
}

// Function to detect message and flagging spam message
func deleteSpamMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
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
			responseSpam := fmt.Sprintf("Spam message detected: **\"%s\"**.", matchedWord)

			// Embed message for simplicity and better view
			embed := &discordgo.MessageEmbed{
				Title:       "Spam Message Detected",
				Description: responseSpam,
				Color:       0xff0000,
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:   "User",
						Value:  m.Author.Username,
						Inline: true,
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
			userID := m.Author.ID
			userName := m.Author.Username

			if guildID == "" {
				fmt.Println("Guild ID cant be found, can't ban member")
				return
			}

			// Tagging username
			err = s.GuildBanCreateWithReason(m.GuildID, m.Author.ID, "spamming detected", 0)
			if err != nil {
				fmt.Printf("Error when banning user %s: %v\n\n", userName, err)
			} else {
				fmt.Printf("User %s has beed banned for spamming\n", userName)

				// Send message to channel about banned member
				banMessageChannel := fmt.Sprintf("User <@%s> (%s) has been banned from server", userID, userName)
				_, err := s.ChannelMessageSend(m.ChannelID, banMessageChannel)
				if err != nil {
					fmt.Println("Error when sending banned message:", err)
				} else {
					fmt.Println("Success sending banned message")
				}
			}
			return
		}
	}
}
