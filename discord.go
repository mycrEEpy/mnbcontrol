package main

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

func (control *Control) handleDiscordMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// don't talk to yourself :)
	if m.Author.ID == s.State.User.ID {
		return
	}

	member, err := s.GuildMember(*discordGuildID, m.Author.ID)
	if err != nil {
		_, err := s.ChannelMessageSend(m.ChannelID, "You are not allowed to talk to me!")
		if err != nil {
			log.Errorf("discord: failed to reply to user %s: %s", m.Member.User.Username, err)
		}
		return
	}

	if !memberHasRole(member, *discordRoleID) {
		_, err := s.ChannelMessageSend(m.ChannelID, "You are not allowed to talk to me!")
		if err != nil {
			log.Errorf("discord: failed to reply to user %s: %s", m.Member.User.Username, err)
		}
		return
	}

	switch m.Content {
	case "!server list":
		managedServers, err := control.listServers(context.Background())
		if err != nil {
			log.Errorf("failed to list server for bot: %s", err)
			_, err := s.ChannelMessageSend(m.ChannelID, "Boom! Something did not work out...")
			if err != nil {
				log.Errorf("discord: failed to reply to user %s: %s", m.Member.User.Username, err)
			}
			return
		}
		if len(managedServers) == 0 {
			_, err = s.ChannelMessageSend(m.ChannelID, "No servers are online.")
			if err != nil {
				log.Errorf("discord: failed to reply to user %s: %s", m.Member.User.Username, err)
			}
			return
		}
		for _, server := range managedServers {
			_, err = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s - %s - %v", server.Name, server.Status, *server.ServerType))
			if err != nil {
				log.Errorf("discord: failed to reply to user %s: %s", m.Member.User.Username, err)
				return
			}
		}
	default:
		_, err := s.ChannelMessageSend(m.ChannelID, "Sorry, I'm not able to help you.")
		if err != nil {
			log.Errorf("discord: failed to reply to user %s: %s", m.Member.User.Username, err)
		}
		return
	}
}

func memberHasRole(member *discordgo.Member, role string) bool {
	var hasRole bool
	for _, r := range member.Roles {
		if r == role {
			hasRole = true
			break
		}
	}
	return hasRole
}
