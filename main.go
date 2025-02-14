package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

var (
	Token            string
	YoutubeAPIKey    string
	YoutubeChannelID string
	DiscordChannelID string
	checkInterval    = 20 * time.Second
	spamRegexPattern = []string{
		`(?i)\b(?:free|get|claim|gift)[\s\+](?:steam|gift|key|cards)[\s\+](giveaway|for free|gratis)\b`,
		`(?i)\b(?:free|get|claim|gift)[\s\+](?:steam|gift|key|cards)\b`,
		`(?i)\bgift[\s-]*cards?\b`,
		`(?i)\b(?:free|get|claim)[\s\+](?:steam|gift|key)[\s\+](?:giveaway|for free|gratis)\b`, // Giveaway Scam (Diperbaiki dan dipersempit)
		`(?i)\btrade\s*offer\b`,
		`(?i)\b(?:free|best|onlyfans|teen)[\s\+](?:porn|NSFW|hub|onlyfans|teen)\b`,
		`(?i)\b(?:stake|airdrop)[\s\+](?:stake|airdrop)\b`,
		`(?i)\bfree\s+\$\d+\b`,
		`(?i)\b(crypto\s+giveaway|eth\s+giveaway|btc\s+giveaway)\b`,
	}
	// Slice to store regex pattern
	compiledRegex []*regexp.Regexp
)

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

type LiveStreamResponse struct {
	Items []struct {
		Id struct {
			VideoId string `json:"videoId"`
		} `json:"id"`
		Snippet struct {
			LiveBroadcastContent string `json:"liveBroadcastContent"`
			Title                string `json:"title"`
		} `json:"snippet"`
	} `json:"items"`
}

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file, using environment variables directly")
		// If .env fails to load, it will try to get from system env variables anyway.
		// This is for deployment environments where .env file is not used.
	}

	Token = os.Getenv("DISCORD_BOT_TOKEN")
	YoutubeAPIKey = os.Getenv("YOUTUBE_API_KEY")
	YoutubeChannelID = os.Getenv("YOUTUBE_CHANNEL_ID")
	DiscordChannelID = os.Getenv("DISCORD_CHANNEL_ID")

	// Discord bot Session
	session, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("Error Discord Session", err)
		return
	}

	session.AddHandler(deleteSpamMessage)

	err = session.Open()
	if err != nil {
		log.Println("Error when connecting", err)
		return
	}
	fmt.Println("Bot working")

	// goroutine for checkLiveStream
	go checkLiveStream(session)

	// Kill discord bot
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	err = session.Close()
	if err != nil {
		log.Println("Cant kill discord")
	}
}

// Function for check livestream
func checkLiveStream(s *discordgo.Session) {
	isCurrentLive := false
	client := &http.Client{Timeout: 19 * time.Second}

	for {
		apiUrl := fmt.Sprintf("https://www.googleapis.com/youtube/v3/search?part=snippet&channelId=%s&eventType=live&type=video&key=%s",
			YoutubeChannelID,
			YoutubeAPIKey,
		)
		respond, err := client.Get(apiUrl)
		if err != nil {
			log.Printf("Error when calling YoutubeAPI: %v, URL: %s", err, apiUrl)
			time.Sleep(checkInterval)
			continue
		}

		if respond.StatusCode != http.StatusOK {
			log.Printf("YoutubeAPI request failed, status code: %d, URL: %s", respond.StatusCode, apiUrl)
			time.Sleep(checkInterval)
			continue
		}

		var searchResponse LiveStreamResponse
		if err := json.NewDecoder(respond.Body).Decode(&searchResponse); err != nil {
			fmt.Println("Error when decoding Youtube API response", err)
			err := respond.Body.Close()
			if err != nil {
				return
			}
			time.Sleep(checkInterval)
			continue
		}
		err = respond.Body.Close()
		if err != nil {
			return
		}

		isLiveNow := len(searchResponse.Items) > 0

		if isLiveNow && !isCurrentLive {
			video := searchResponse.Items[0]
			liveVideoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", video.Id.VideoId)
			messageLive := fmt.Sprintf(" Geeons %s %s",
				video.Snippet.LiveBroadcastContent,
				video.Snippet.Title,
				liveVideoURL)

			_, err := s.ChannelMessageSend(DiscordChannelID, messageLive)
			if err != nil {
				fmt.Println("Error when sending message", err)
			} else {
				fmt.Println("Live stream detected, send message to Discord channel")
			}
		}

		isCurrentLive = isLiveNow
		time.Sleep(checkInterval)
	}
}

func deleteSpamMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	fmt.Println("Message Recieved:", m.Content)

	for _, regex := range compiledRegex {
		if regex.MatchString(m.Content) {
			log.Printf("Spam detected in message from %s", m.Author.Username)

			deleteMessageContent := m.Content

			responseSpam := fmt.Sprintf("Spam message detected: \"%s\".", deleteMessageContent)
			_, err := s.ChannelMessageSendReply(m.ChannelID, responseSpam, m.Reference())
			if err != nil {
				fmt.Println("Failed to send delete message", err)
			}

			err = s.ChannelMessageDelete(m.ChannelID, m.ID)
			if err != nil {
				fmt.Println("Error when trying to delete message", err)
			}

			// Banned spam chats member
			guildID := m.GuildID
			userID := m.Author.ID
			userName := m.Author.Username

			if guildID == "" {
				fmt.Println("Guild ID cant be found, can't ban member")
				return
			}

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
