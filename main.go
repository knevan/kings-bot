package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"

	"kings-bot/antiscam"
	"kings-bot/slashcommands"
	"kings-bot/youtube"
)

var (
	Token            string
	VerifyToken      string
	YoutubeAPIKey    string
	YoutubeChannelID string
	DiscordChannelID string
	BanLogChannelID  string
	KingKongRoleID   string

	// Port Server for running the server
	Port = "8080"
)

func main() {
	// Load environment variables from .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Error loading .env file, using environment variables directly")
	}

	// Get Token from .env file
	Token = os.Getenv("DISCORD_BOT_TOKEN")
	YoutubeAPIKey = os.Getenv("YOUTUBE_API_KEY")
	YoutubeChannelID = os.Getenv("YOUTUBE_CHANNEL_ID")
	DiscordChannelID = os.Getenv("DISCORD_CHANNEL_ID")
	VerifyToken = os.Getenv("VERIFY_TOKEN")
	BanLogChannelID = os.Getenv("BAN_LOG_CHANNEL_ID")
	KingKongRoleID = os.Getenv("ROLE_KINGKONG_ID")

	// Discord bot Session
	session, err := discordgo.New("Bot " + Token)
	if err != nil {
		fmt.Println("Error Discord Session", err)
		return
	}

	// Connecting bot with bot token
	err = session.Open()
	if err != nil {
		log.Println("Error when connecting", err)
		return
	}
	fmt.Println("Bot working")
	// Initialize slashcommands module
	antiscam.Init(BanLogChannelID)

	// Handler for Spam Message
	session.AddHandler(antiscam.DeleteSpamMessage)

	// Handler for Slash Command
	session.AddHandler(slashcommands.UnbanhandlerCommand)
	// Register unban command
	_, err = session.ApplicationCommandCreate(session.State.User.ID, "", slashcommands.UnbanCommand)
	if err != nil {
		log.Printf("Error creating unban command: %v", err)
	}

	// Initialize Youtube Module
	youtube.Init(DiscordChannelID, VerifyToken, YoutubeAPIKey, KingKongRoleID)

	// Setup http server for YouTube Webhook
	http.HandleFunc("/youtube/webhook", func(w http.ResponseWriter, r *http.Request) {
		youtube.HandleYoutubeWebhook(w, r, session)
	})

	// Subscribe youtube channel
	err = youtube.SubscribeYoutubeChannel(YoutubeChannelID)
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

	ticker := time.NewTicker(345600 * time.Second)
	go func() {
		for range ticker.C {
			if err := youtube.SubscribeYoutubeChannel(YoutubeChannelID); err != nil {
				log.Printf("Error resubscribing %v", err)
			} else {
				log.Printf("Success resubscribing %v", err)
			}
		}
	}()

	// Kill discord bot
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanup and Close bot session
	err = session.Close()
	if err != nil {
		log.Println("Cant kill discord")
	}
}
