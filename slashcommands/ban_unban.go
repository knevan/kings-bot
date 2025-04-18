package slashcommands

import (
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"

	"kings-bot/db"
)

// variable used across the package
var (
	banLogChannelID string
	defaultPerms    int64 = discordgo.PermissionBanMembers // Minimum permission required
	defaultDM             = false                          // disable command in DM
)

// UnbanCommand : Represents the Discord application command for unban functionality
var UnbanCommand = &discordgo.ApplicationCommand{
	Name:        "unban",
	Description: "Unban user",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "user",
			Description: "Insert User ID",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "reason",
			Description: "Insert Reason",
			Required:    true,
		},
	},
	DefaultMemberPermissions: &defaultPerms, // User must have permission to ban member
	DMPermission:             &defaultDM,    // Disable command in DM
}

// UnbanhandlerCommand : Handle the unban command invoke by user
func UnbanhandlerCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Check permission or required role
	if !hasRequiredRole(s, i.GuildID, i.Member) {
		respondWithError(s, i, "You dont have permission to use this command")
		return
	}

	options := i.ApplicationCommandData().Options
	userID := options[0].StringValue()
	reason := options[1].StringValue()

	if len(options) > 1 {
		reason = options[1].StringValue()
	}

	// Check if the user is still banned
	bans, err := s.GuildBans(i.GuildID, 1000, "", "")
	if err != nil {
		respondWithError(s, i, fmt.Sprintf("Failed to retrieve ban list: %v", err))
		return
	}

	userIsBanned := false
	for _, ban := range bans {
		if ban.User.ID == userID {
			userIsBanned = true
			break
		}
	}

	if !userIsBanned {
		respondWithError(s, i, "This user is not currently banned.")
		return
	}

	err = s.GuildBanDelete(i.GuildID, userID) // Remove the ban
	if err != nil {
		respondWithError(s, i, fmt.Sprintf("Failed to unban user: %v", err))
		return
	}

	// Embed message for confirmation unban
	embed := &discordgo.MessageEmbed{
		Title: "User Unbanned",
		Color: 0x00ff00,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "User ID",
				Value:  userID,
				Inline: true,
			},
			{
				Name:   "Unbanned by",
				Value:  i.Member.User.Username,
				Inline: true,
			},
			{
				Name:   "Reason",
				Value:  reason,
				Inline: false,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "User Unbanned",
		},
	}

	// Send respond confirmation for successful unban command
	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
	if err != nil {
		return
	}
}

// Check if member has required MOD role
func hasRequiredRole(s *discordgo.Session, guildID string, member *discordgo.Member) bool {
	for _, roleID := range member.Roles {
		role, err := s.State.Role(guildID, roleID)
		if err != nil {
			continue
		}
		if role.Name == "MOD" {
			return true
		}
	}
	return false
}

// Send ephemeral error message in response to a slash command
func respondWithError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral, // Hide message from other user
		},
	})
	if err != nil {
		return
	}
}

// BanCommand : variable for discord bot command ban functionality
var BanCommand = &discordgo.ApplicationCommand{
	Name:        "ban",
	Description: "Ban user",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "user",
			Description: "Insert UserID",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "time",
			Description: "Ban duration in hours (0 for perma), max 720hr(30d)",
			Required:    true,
			MinValue:    &[]float64{0}[0], // Minimum duration 0 for perma ban
			MaxValue:    720,              // Max 30D ban
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "delmsg",
			Description: "Delete old message (0-7)",
			Required:    true,
			MinValue:    &[]float64{0}[0],
			MaxValue:    7,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "reason",
			Description: "Insert Reason",
			Required:    false,
		},
	},
	DefaultMemberPermissions: &defaultPerms, // Require ban permission role
	DMPermission:             &defaultDM,    // Disable command in DM
}

// InitBan Init the ban log channel
func InitBan(logChannelID string) {
	banLogChannelID = logChannelID
}

// Fetch banned user information (username)
func getBannedUserInfo(s *discordgo.Session, userID string) (*discordgo.User, error) {
	return s.User(userID)
}

// BanhandlerCommand : Handle the ban command when invoke by user
func BanhandlerCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Check permission or required role
	if !hasRequiredRole(s, i.GuildID, i.Member) {
		respondWithError(s, i, "You dont have permission to use this command")
		return
	}

	options := i.ApplicationCommandData().Options
	userID := options[0].StringValue()
	banDurationHours := options[1].IntValue()
	deleteMsgDays := int(options[2].IntValue())

	var reason string
	if len(options) > 3 && options[3].StringValue() != "" {
		reason = options[3].StringValue()
	} else {
		reason = "No reason provided"
	}

	// Calculate number of days
	banDuration := time.Duration(banDurationHours) * time.Hour

	// Determine ban duration string
	var durationString string
	if banDurationHours == 0 {
		durationString = "permanently"
	} else {
		durationString = fmt.Sprintf("%d Hours", banDurationHours)
	}

	// Calculate ban end time
	var banEndTime time.Time
	if banDurationHours > 0 {
		banEndTime = time.Now().Add(banDuration)
	}
	banUnixTime := banEndTime.Unix()

	// Send DM message to banned user
	sendDMMessage(s, i, userID, banDurationHours, reason, banUnixTime)

	// Fetch banned username for log message
	bannedUsername, err := getBannedUserInfo(s, userID)
	if err != nil {
		respondWithError(s, i, fmt.Sprintf("Failed to retrieve banned user info: %v", err))
		return
	}

	// Ban the user from server
	err = s.GuildBanCreateWithReason(i.GuildID, userID, reason, deleteMsgDays)
	if err != nil {
		respondWithError(s, i, fmt.Sprintf("Failed to ban user: %v", err))
		return
	}

	// Add temporary ban to database to track timed bans
	if banDurationHours > 0 {
		err = db.AddTempBan(userID, i.GuildID, i.Member.User.ID, banDuration, reason)
		if err != nil {
			log.Printf("Error adding temporary ban to database: %v", err)
		}
	}

	// Create ember for log message
	logEmbed := &discordgo.MessageEmbed{
		Title: "User Banned by MOD from KinG Server",
		Color: 0xff0000,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: bannedUsername.AvatarURL(""),
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Username",
				Value:  bannedUsername.Username,
				Inline: true,
			},
			{
				Name:   "User ID",
				Value:  userID,
				Inline: true,
			},
			{
				Name:   "Reason",
				Value:  reason,
				Inline: false,
			},
			{
				Name:   "Banned by",
				Value:  i.Member.User.Username,
				Inline: true,
			},
			{
				Name:   "Duration",
				Value:  fmt.Sprintf("%s, <t:%d:R>", durationString, banEndTime.Unix()),
				Inline: true,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Send ban log to specific channel
	_, err = s.ChannelMessageSendEmbed(banLogChannelID, logEmbed)
	if err != nil {
		respondWithError(s, i, fmt.Sprintf("Failed to send log message: %v", err))
		return
	}

	// Send ban confirmation message to channel
	var message string
	if banDurationHours > 0 {
		message = fmt.Sprintf("User %s has been banned for <t:%d:R>. Reason: %s",
			bannedUsername.Username, banUnixTime, reason)
	} else {
		message = fmt.Sprintf("User %s has been banned permanently. Reason: %s",
			bannedUsername.Username, reason)
	}

	err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
		},
	})
	if err != nil {
		respondWithError(s, i, fmt.Sprintf("Failed to send message: %v", err))
	}
}

// Send DirectMessage to Banned User
func sendDMMessage(s *discordgo.Session, i *discordgo.InteractionCreate, userID string, banDurationHours int64, reason string, banUnixTime int64) {
	// Create a single-use, never-expiring dicord invite link
	invite, err := s.ChannelInviteCreate(i.ChannelID, discordgo.Invite{
		MaxAge:    0,
		MaxUses:   1,
		Temporary: false,
	})
	if err != nil {
		log.Printf("Error creating Discord invite: %v", err)
	}

	// Create DM Embed Message for Temporary Ban
	var dmEmbed *discordgo.MessageEmbed
	if banDurationHours > 0 {
		dmEmbed = &discordgo.MessageEmbed{
			Title: "You have been **Temporarily** banned from KinG server",
			Description: fmt.Sprintf("You have been banned until <t:%d:F> <t:%d:R> due to Spamming and Compromissed Account. \n \n"+
				"If you have gained access and secured your account, you can rejoin after the ban period using this one time invite link:", banUnixTime, banUnixTime),
			Color: 0xff0000,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:  "Reason",
					Value: reason,
				},
			},
		}
	} else {
		dmEmbed = &discordgo.MessageEmbed{
			Title:       "You have been **Permanently** banned from KinG server",
			Description: "You have been permanently due to Spamming and Compromissed Account.",
			Color:       0xff0000,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:  "Reason",
					Value: reason,
				},
			},
		}
	}

	// Send DM message to banned user
	dmChannel, err := s.UserChannelCreate(userID)
	if err != nil {
		log.Printf("Error creating DM channel for banned user: %v", err)
	} else {
		_, err = s.ChannelMessageSendEmbed(dmChannel.ID, dmEmbed)
		if err != nil {
			log.Printf("Failed to send DM message to banned user: %v", err)
		}

		if banDurationHours > 0 && invite != nil {
			inviteMessage := fmt.Sprintf("https://discord.gg/%s", invite.Code)
			_, err = s.ChannelMessageSend(dmChannel.ID, inviteMessage)
			if err != nil {
				log.Printf("Failed to send invite message to DM channel: %v", err)
			}
		}
	}
}
