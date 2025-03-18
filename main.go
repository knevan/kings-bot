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
	"kings-bot/db"
	"kings-bot/slashcommands"
	"kings-bot/youtube"
)

var (
	Token                        string
	VerifyToken                  string
	YoutubeAPIKey                string
	YoutubeChannelID             string
	YoutubeNotificationChannelID string
	BanLogChannelID              string
	KingKongRoleID               string

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
	VerifyToken = os.Getenv("VERIFY_TOKEN")
	YoutubeChannelID = os.Getenv("YOUTUBE_CHANNEL_ID")
	YoutubeNotificationChannelID = os.Getenv("YOUTUBE_NOTIFICATION_CHANNEL_ID")
	BanLogChannelID = os.Getenv("BAN_LOG_CHANNEL_ID")
	KingKongRoleID = os.Getenv("ROLE_KINGKONG_ID")

	// Initialize Database
	err = db.InitDB()
	if err != nil {
		log.Fatalf("Error initilizing database: %v", err)
	}

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

	// Initialize antiscam init module
	antiscam.Init(BanLogChannelID)

	// Handler for Spam Message
	session.AddHandler(antiscam.DeleteSpamMessage)

	// Initialize slashcommands init module
	slashcommands.InitBan(BanLogChannelID)

	// Handler for Slash Commands
	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		switch i.Type {
		case discordgo.InteractionApplicationCommand:
			switch i.ApplicationCommandData().Name {
			case "unban":
				slashcommands.UnbanhandlerCommand(s, i)
			case "ban":
				slashcommands.BanhandlerCommand(s, i)
			}
		}
	})

	_, err = session.ApplicationCommandCreate(session.State.User.ID, "", slashcommands.UnbanCommand)
	if err != nil {
		log.Printf("Error creating unban command: %v", err)
	}

	_, err = session.ApplicationCommandCreate(session.State.User.ID, "", slashcommands.BanCommand)
	if err != nil {
		log.Printf("Error creating ban command: %v", err)
	}

	// Start database ban ticker
	go banDatabaseTicker(session)

	// Initialize Youtube Module
	youtube.Init(YoutubeNotificationChannelID, VerifyToken, YoutubeAPIKey, KingKongRoleID)

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

// Function to check database ban status and unban if time is up
func banDatabaseTicker(s *discordgo.Session) {
	ticker := time.NewTicker(3 * time.Minute)
	for range ticker.C {
		expiredBans, err := db.GetExpiredBans()
		if err != nil {
			log.Printf("Error getting expired bans: %v", err)
			continue
		}
		for _, ban := range expiredBans {
			err := s.GuildBanDelete(ban.GuildID, ban.UserID)
			if err != nil {
				log.Printf("Error unbanning user %s from guild %s: %v", ban.UserID, ban.GuildID, err)
			} else {
				log.Printf("Unbanned user %s from guild %s", ban.UserID, ban.GuildID)
			}
			err = db.RemoveTempBans(ban.UserID)
			if err != nil {
				log.Printf("Error removing temporary ban for user %s: %v", ban.UserID, err)
			}
		}
	}
}
