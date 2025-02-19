package youtube

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

var (
	channelID string
	// discordChannel string
	verifyToken   string
	youtubeAPIKey string

	inMemoryCache       = make(map[string]time.Time)
	cacheMutex          sync.Mutex
	cacheExpirationTime = 8 * time.Hour
)

// Notification XMLFeed Notification YoutubeNotification struct for xml payload from YouTube
type Notification struct {
	XMLName xml.Name `xml:"feed"`
	Title   string   `xml:"title"`
	Updated string   `xml:"updated"`
	Entry   Entry    `xml:"entry"`
}

type Entry struct {
	ID        string `xml:"id"`
	VideoID   string `xml:"videoId"`
	ChannelID string `xml:"channelId"`
	Title     string `xml:"title"`
}

// Init initializes the YouTube module
func Init(youtubeChannelID string, verifyTokenValue string, youtubeKey string) {
	channelID = youtubeChannelID
	// discordChannel = discordChannelID
	verifyToken = verifyTokenValue
	youtubeAPIKey = youtubeKey
}

// HandleYoutubeWebhook Handle Webhook
func HandleYoutubeWebhook(w http.ResponseWriter, r *http.Request, s *discordgo.Session) {
	log.Printf("Received webhook request: %s %s", r.Method, r.URL.Path)

	// Handle verification GET request
	if r.Method == "GET" {
		challenge := r.URL.Query().Get("hub.challenge")
		verifyTokenRecieved := r.URL.Query().Get("hub.verify_token")

		// log.Printf("Verification attempt - Token: %s, Challenge: %s", verifyToken, challenge)
		if verifyTokenRecieved != verifyToken {
			log.Printf("Invalid verify token received: %s", verifyTokenRecieved)
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

		var notification Notification
		if err := xml.Unmarshal(body, &notification); err != nil {
			http.Error(w, "Error parsing XML youtube notification", http.StatusBadRequest)
		}

		log.Printf("Received YouTube notification: %+v", notification)

		if !alreadyProcessedVideo(notification.Entry.VideoID) {
			live, err := isLiveStream(notification.Entry.VideoID)
			if err != nil {
				log.Printf("Error checking live status: %v", err)
			}

			if live {
				message := fmt.Sprintf("@everyone %s is live! Watch here: https://www.youtube.com/watch?v=%s",
					notification.Entry.Title, notification.Entry.VideoID)
				_, err := s.ChannelMessageSend(channelID, message)
				if err != nil {
					log.Printf("Error sending Discord message: %v", err)
				} else {
					log.Printf("Sent Discord message: %s", notification.Entry.Title)
					markVideoAsProcessed(notification.Entry.VideoID)
				}
			} else {
				log.Printf("Video %s is not a live, skipping notification.", notification.Entry.VideoID)
			}
		} else {
			log.Printf("Video %s has already been processed, skipping notification.", notification.Entry.VideoID)
		}
		w.WriteHeader(http.StatusOK)
	}
}

func isLiveStream(videoID string) (bool, error) {
	log.Printf("Checking live status for video ID: %s", videoID)
	ctx := context.Background()
	service, err := youtube.NewService(ctx, option.WithAPIKey(youtubeAPIKey))
	if err != nil {
		return false, fmt.Errorf("error creating YouTube service: %v", err)
	}

	call := service.Videos.List([]string{"snippet"}).Id(videoID)
	resp, err := call.Do()
	if err != nil {
		return false, fmt.Errorf("error fetching video info defailt: %v", err)
	}

	if len(resp.Items) == 0 {
		return false, fmt.Errorf("video not found")
	}

	liveStatus := resp.Items[0].Snippet.LiveBroadcastContent
	log.Printf("Live status for video: %s", liveStatus)
	return liveStatus == "live", nil
}

func alreadyProcessedVideo(videoID string) bool {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	if t, exists := inMemoryCache[videoID]; exists {
		if time.Since(t) < cacheExpirationTime {
			return true
		}
		delete(inMemoryCache, videoID)
	}
	inMemoryCache[videoID] = time.Now()
	return false
}

func markVideoAsProcessed(videoID string) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()
	inMemoryCache[videoID] = time.Now()
}

func SubscribeYoutubeChannel(channelID string) error {
	callbackURL := fmt.Sprintf("https://13f6-180-252-117-209.ngrok-free.app/youtube/webhook")
	topicURL := fmt.Sprintf("https://www.youtube.com/xml/feeds/videos.xml?channel_id=%s", channelID)

	values := url.Values{}
	values.Set("hub.callback", callbackURL)
	values.Set("hub.topic", topicURL)
	values.Set("hub.verify_token", verifyToken)
	values.Set("hub.mode", "subscribe")
	values.Set("hub.lease_seconds", "432000")

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
