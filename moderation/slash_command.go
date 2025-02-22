package moderation

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

var BanCommand = &discordgo.ApplicationCommand{
	Name:        "ban",
	Description: "Ban User",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:         discordgo.ApplicationCommandOptionUser,
			Name:         "user",
			Description:  "Insert User ID",
			Required:     true,
			Autocomplete: true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "reason",
			Description: "Insert Reason",
			Required:    true,
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "duration",
			Description: "Insert Duration (in seconds)",
			Required:    false,
			MinValue:    &[]float64{0}[0],
			MaxValue:    7,
		},
	},
}

func BanhandlerCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if !hasRequiredRole(s, i.GuildID, i.Member) {
		respondWithError(s, i, "You dont have permission to use this command")
		return
	}

	options := i.ApplicationCommandData().Options
	user := options[0].UserValue(s)
	reason := options[1].StringValue()
	deleteDays := 0

	if len(options) > 2 {
		deleteDays = int(options[2].IntValue())
	}

	err := s.GuildBanCreateWithReason(i.GuildID, user.ID, reason, deleteDays)
	if err != nil {
		respondWithError(s, i, fmt.Sprintf("Failed to ban user: %v", err))
		return
	}

	embed := &discordgo.MessageEmbed{
		Title: "User Banned",
		Color: 0xFF0000,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "User",
				Value:  user.Username + "#" + user.Discriminator,
				Inline: true,
			},
			{
				Name:   "Banned By",
				Value:  i.Member.User.Username,
				Inline: true,
			},
			{
				Name:   "Reason",
				Value:  reason,
				Inline: false,
			},
			{
				Name:   "Duration",
				Value:  fmt.Sprintf("%d days", deleteDays),
				Inline: true,
			},
		},
		Timestamp: time.Now().Format(time.RFC3339),
		Footer: &discordgo.MessageEmbedFooter{
			Text: "User Banned",
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
}

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

	err := s.GuildBanDelete(i.GuildID, userID)
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
		if role.Name == "MOD" || role.Name == "Moderator" {
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
