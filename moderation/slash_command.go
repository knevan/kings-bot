package moderation

import (
	"fmt"
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	defaultPerms int64 = discordgo.PermissionBanMembers
	defaultDM          = false
)

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
