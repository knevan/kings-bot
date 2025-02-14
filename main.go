package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
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
	// Token            = os.Getenv("DISCORD_BOT_TOKEN")
	// YoutubeAPIKey    = os.Getenv("YOUTUBE_API_KEY")
	// YoutubeChannelID = os.Getenv("YOUTUBE_CHANNEL_ID")
	// DiscordChannelID = os.Getenv("DISCORD_CHANNEL_ID")
	checkInterval = 20 * time.Second
	scamWords     = []string{"gift-card", "steamcommunity"}
	pornWords     = []string{"porn", "+18"}
)

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
	client := &http.Client{Timeout: 20 * time.Second}

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
			// liveVideoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", video.Id.VideoId)
			messageLive := fmt.Sprintf(" Geeons %s %s",
				video.Snippet.LiveBroadcastContent,
				video.Snippet.Title)
			// liveVideoURL)

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

	for _, word := range scamWords {
		if strings.Contains(strings.ToLower(m.Content), word) {
			fmt.Println("steamcommunity detected!")

			responseScam := "Scam Message"
			_, err := s.ChannelMessageSend(m.ChannelID, responseScam)
			if err != nil {
				fmt.Println("Failed", err)
			}

			err = s.ChannelMessageDelete(m.ChannelID, m.ID)
			if err != nil {
				fmt.Println("Error when trying to delete message", err)
			}
		}
	}
}
