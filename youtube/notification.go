package youtube

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/bwmarrin/discordgo"
)

var (
	channelID      string
	discordChannel string
	verifyToken    string
	youtubeAPIKey         string
)

// Notification XMLFeed Notification YoutubeNotification struct for xml payload from YouTube
type Notification struct {
	XMLName xml.Name `xml:"feed"`
	Entry   struct {
		VideoID   string `xml:"videoID"`
		ChannelID string `xml:"channelID"`
		Title     string `xml:"title"`
	} `xml:"entry"`
}

// Init initializes the YouTube module
func Init(youtubeChannelID string, discordChannelID string, verifyTokenValue string, youtubeKey string) {
	channelID = youtubeChannelID
	discordChannel = discordChannelID
	verifyToken = verifyTokenValue
	youtubeKey = youtubeAPIKey
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

		// Check if video is livestream
		if isLiveStream(notification.Entry.VideoID) {
		    message := fmt.Sprintf("@everyone %s Live! Watch here: https://www.youtube.com/watch?v=%s", notification.Entry.Title,
				notification.Entry.VideoID)

			_, err := s.ChannelMessageSend(discordChannel, message)
			if err != nil {
                log.Printf("Error sending notification message: %v", err)
            } else {
                log.Printf("Sent notification message: %v", notification.Entry.Title)
            }
	}
		/*
		if liveStatus == "live" || liveStatus == "upcoming" {
			message := fmt.Sprintf("@everyone %s Live! Watch here: https://www.youtube.com/watch?v=%s", notification.Entry.Title,
				notification.Entry.VideoID)

			_, err := s.ChannelMessageSend(discordChannel, message)
			if err != nil {
				log.Printf("Error sending notification message: %v", err)
			} else {
				log.Printf("Sent notification message: %v", notification.Entry.Title)
			}
		}*/

		/* // Check if channel start Livestream
		log.Printf("Recieved notification: %v", notification)
		if notification.Entry.Status.Type == "ive" {
			message := fmt.Sprintf("@everyone %s \\nhttps://youtube.com/watch?v=%s\\n",
				notification.Entry.Title,
				notification.Entry.VideoID)

			_, err = s.ChannelMessageSend(discordChannel, message)
			if err != nil {
				log.Printf("Error Sending Discord Message: %v", err)
			} else {
				log.Printf("%s", notification.Entry.Title)
			}
		}*/
		w.WriteHeader(http.StatusOK)
}

func SubscribeYoutubeChannel(channelID string) error {
	callbackURL := fmt.Sprintf("https://6909-180-252-117-209.ngrok-free.app/youtube/webhook")
	topicURL := fmt.Sprintf("https://www.youtube.com/xml/feeds/videos.xml?channel_id=%s", channelID)

	values := url.Values{}
	values.Set("hub.callback", callbackURL)
	values.Set("hub.topic", topicURL)
	values.Set("hub.verify_token", verifyToken)
	values.Set("hub.mode", "subscribe")
	values.Set("hub.lease_seconds", "432000")

	resp, err := http.PostForm("https://pubsubhubbub.appspot.com/subscribe", values)
	if err != nil{
		return
	} fmt.Errorf("error subscribing: %v", err)

	defer func (){
		if err := resp.Body.Close(); err != nil{
			log.Printf("Error closing body: %v", err)
	}
	}()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK{
		return fmt.Errorf("something error in here: %d", resp.StatusCode)
	}
	log.Printf("Success subscribe to channel: %s", channelID)
	return nil
}
