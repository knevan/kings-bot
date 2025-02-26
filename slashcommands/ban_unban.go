package slashcommands

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	defaultPerms    int64 = discordgo.PermissionBanMembers
	defaultDM             = false
	banLogChannelID string
)

// UnbanCommand Variable for Unban Command
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
	DefaultMemberPermissions: &defaultPerms,
	DMPermission:             &defaultDM,
}

func Init(logChannelID string) {
	banLogChannelID = logChannelID
}

// UnbanhandlerCommand Function to handle Unban Command
func UnbanhandlerCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
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

	// Check if the user is actually banned
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

	err = s.GuildBanDelete(i.GuildID, userID)
	if err != nil {
		respondWithError(s, i, fmt.Sprintf("Failed to unban user: %v", err))
		return
	}

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

func respondWithError(s *discordgo.Session, i *discordgo.InteractionCreate, message string) {
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: message,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		return
	}
}

// BanCommand variable for Ban Command
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
			Description: "Ban duration in hours (0 for permanent ban)",
			Required:    true,
			MinValue:    &[]float64{0}[0],
			MaxValue:    720,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "reason",
			Description: "Insert Reason",
			Required:    false,
		},
	},
	DefaultMemberPermissions: &defaultPerms,
	DMPermission:             &defaultDM,
}

func getBannedUserInfo(s *discordgo.Session, userID string) (*discordgo.User, error) {
	return s.User(userID)
}

func BanhandlerCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !hasRequiredRole(s, i.GuildID, i.Member) {
		respondWithError(s, i, "You dont have permission to use this command")
		return
	}

	options := i.ApplicationCommandData().Options
	userID := options[0].StringValue()
	bandurationHours := options[1].IntValue()

	var reason string
	if len(options) > 2 {
		reason = options[2].StringValue()
	}

	banDurationDays := int(bandurationHours / 24)
	if bandurationHours%24 != 0 {
		banDurationDays++
	}

	err := s.GuildBanCreateWithReason(i.GuildID, userID, reason, banDurationDays)
	if err != nil {
		respondWithError(s, i, fmt.Sprintf("Failed to ban user: %v", err))
		return
	}

	var durationString string
	if bandurationHours == 0 {
		durationString = "permanently"
	} else {
		durationString = fmt.Sprintf("%d Hours", bandurationHours)
	}

	bannedUsername, err := getBannedUserInfo(s, userID)
	if err != nil {
		respondWithError(s, i, fmt.Sprintf("Failed to retrieve banned user info: %v", err))
		return
	}

	logEmbed := &discordgo.MessageEmbed{
		Title: "User Banned by MOD",
		Color: 0xff0000,
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
				Value:  durationString,
				Inline: true,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	_, err = s.ChannelMessageSendEmbed(banLogChannelID, logEmbed)
	if err != nil {
		respondWithError(s, i, fmt.Sprintf("Failed to send log message: %v", err))
		return
	}

	message := fmt.Sprintf("User %s banned for %s. Reason: %s", userID, durationString, reason)
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
