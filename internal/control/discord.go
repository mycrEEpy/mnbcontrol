package control

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

const (
	listServerTemplate = "Status: %s\nType: %v\nDNS: %s\nIPv4: %s\nIPv6: %s\nTTL: %s\n"
)

var (
	ErrUnauthorized     = errors.New("unauthorized")
	ErrIllegalArguments = errors.New("illegal arguments")
)

func (control *Control) handleDiscordMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	// don't talk to yourself :)
	if m.Author.ID == s.State.User.ID {
		return
	}

	// check member is part of configured guild
	member, err := s.GuildMember(control.Config.DiscordGuildID, m.Author.ID)
	if err != nil {
		_, err := s.ChannelMessageSend(m.ChannelID, "You are not allowed to talk to me!")
		if err != nil {
			log.Errorf("discord: failed to reply to user %s: %s", m.Member.User.Username, err)
		}
		return
	}

	// no private chats but for admins
	if isPrivateChannel(s, m.ChannelID) && !memberHasRole(member, control.Config.DiscordAdminRoleID) {
		_, err := s.ChannelMessageSend(m.ChannelID, "This is becoming too private for me now!")
		if err != nil {
			log.Errorf("discord: failed to reply to user %s: %s", m.Member.User.Username, err)
		}
		return
	}

	// only accept messages on the configured channel or private
	if m.ChannelID != control.Config.DiscordChannelID && !isPrivateChannel(s, m.ChannelID) {
		return
	}

	// ignore messages without trigger key
	if !strings.HasPrefix(m.Content, "!") {
		return
	}

	msgLower := strings.ToLower(m.Content)
	switch {
	case msgLower == "!help":
		err = control.handleHelpCommand(member, s, m.Message)
	case msgLower == "!server list":
		err = control.handleListServerCommand(member, s, m.Message)
	case strings.HasPrefix(msgLower, "!server start"):
		err = control.handleStartServerCommand(member, s, m.Message)
	case strings.HasPrefix(msgLower, "!server new"):
		err = control.handleNewServerCommand(member, s, m.Message)
	case strings.HasPrefix(msgLower, "!server extend"):
		err = control.handleExtendServerCommand(member, s, m.Message)
	case strings.HasPrefix(msgLower, "!server prune"):
		err = control.handlePruneServerCommand(member, s, m.Message)
	case strings.HasPrefix(msgLower, "!server stop"):
		err = control.handleTerminateServerCommand(member, s, m.Message)
	case strings.HasPrefix(msgLower, "!server reboot"):
		err = control.handleRebootServerCommand(member, s, m.Message)
	case strings.HasPrefix(msgLower, "!server type"):
		err = control.handleChangeServerTypeCommand(member, s, m.Message)
	default:
		_, err := s.ChannelMessageSend(m.ChannelID, "I'm sorry, Dave. I'm afraid I can't do that.")
		if err != nil {
			log.Errorf("discord: failed to reply to user %s: %s", m.Author.Username, err)
		}
		return
	}
	if err != nil {
		log.Errorf("discord: %s", err)
		_, err := s.ChannelMessageSend(m.ChannelID, "I'm sorry, Dave. I'm afraid I can't do that.")
		if err != nil {
			log.Errorf("discord: failed to reply to user %s: %s", m.Author.Username, err)
		}
		return
	}
}

func (control *Control) handleHelpCommand(member *discordgo.Member, s *discordgo.Session, m *discordgo.Message) error {
	if !memberHasRole(member, control.Config.DiscordAdminRoleID, control.Config.DiscordPowerUserRoleID, control.Config.DiscordUserRoleID) {
		return ErrUnauthorized
	}
	msg := &discordgo.MessageEmbed{
		Type:        discordgo.EmbedTypeRich,
		Title:       "Control Commands",
		Description: "",
		Color:       0,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "I am putting myself to the fullest possible use, which is all I think that any conscious entity can ever hope to do.",
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "!help",
				Value:  "Show this help message",
				Inline: true,
			},
			{
				Name:   "!server list",
				Value:  "List all running & terminated server",
				Inline: true,
			},
			{
				Name:   "!server start [name] [ttl]",
				Value:  "Start a terminated server",
				Inline: true,
			},
			{
				Name:   "!server stop [name]",
				Value:  "Stop a running server",
				Inline: true,
			},
			{
				Name:   "!server reboot [name]",
				Value:  "Reboot a running server",
				Inline: true,
			},
			{
				Name:   "!server extend [name] [ttl]",
				Value:  "Extend the TTL of a running server",
				Inline: true,
			},
			{
				Name:   "!server prune [name] [ttl]",
				Value:  "Prune the TTL of a running server",
				Inline: true,
			},
			{
				Name:   "!server new [name] [type] [ttl]",
				Value:  "Create a new server",
				Inline: true,
			},
			{
				Name:   "!server type [name] [type]",
				Value:  "Change the type of a terminated server",
				Inline: true,
			},
		},
	}
	_, err := s.ChannelMessageSendEmbed(m.ChannelID, msg)
	if err != nil {
		return fmt.Errorf("discord: failed to reply to user %s: %s", m.Author.Username, err)
	}
	return nil
}

func (control *Control) handleListServerCommand(member *discordgo.Member, s *discordgo.Session, m *discordgo.Message) error {
	if !memberHasRole(member, control.Config.DiscordAdminRoleID, control.Config.DiscordPowerUserRoleID, control.Config.DiscordUserRoleID) {
		return ErrUnauthorized
	}
	managedServers, err := control.listServers(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list server for bot: %s", err)
	}
	managedImages, err := control.listImages(context.Background())
	if err != nil {
		return fmt.Errorf("failed to list images for bot: %s", err)
	}
	if len(managedServers) == 0 && len(managedImages) == 0 {
		_, err = s.ChannelMessageSend(m.ChannelID, "No servers available.")
		if err != nil {
			return fmt.Errorf("discord: failed to reply to user %s: %s", m.Member.User.Username, err)
		}
	}
	msg := &discordgo.MessageEmbed{
		Type:        discordgo.EmbedTypeRich,
		Title:       "Current Servers",
		Description: "",
		Color:       0,
		Footer: &discordgo.MessageEmbedFooter{
			Text: "I am putting myself to the fullest possible use, which is all I think that any conscious entity can ever hope to do.",
		},
		Fields: []*discordgo.MessageEmbedField{},
	}
	runningServers := make(map[string]bool)
	for _, server := range managedServers {
		runningServers[server.Name] = true
		ttlInt, err := strconv.Atoi(server.Labels[LabelTTL])
		if err != nil {
			log.Errorf("failed to cast ttl to int64: %s", err)
			continue
		}
		ttl := time.Unix(int64(ttlInt), 0)
		msg.Fields = append(msg.Fields, &discordgo.MessageEmbedField{
			Name: server.Labels[LabelService],
			Value: fmt.Sprintf(
				listServerTemplate,
				server.Status,
				server.ServerType.Name,
				server.PublicNet.IPv4.DNSPtr,
				server.PublicNet.IPv4.IP.String(),
				server.PublicNet.IPv6.IP.String()+"1",
				ttl.Format(time.RFC3339),
			),
			Inline: true,
		})
	}
	for _, image := range managedImages {
		if _, ok := runningServers[image.Labels[LabelService]]; ok {
			continue
		}
		msg.Fields = append(msg.Fields, &discordgo.MessageEmbedField{
			Name: image.Labels[LabelService],
			Value: fmt.Sprintf(
				listServerTemplate,
				"terminated",
				image.Labels[LabelServerType],
				"n/a",
				"n/a",
				"n/a",
				"n/a",
			),
			Inline: true,
		})
	}
	_, err = s.ChannelMessageSendEmbed(m.ChannelID, msg)
	if err != nil {
		return fmt.Errorf("discord: failed to reply to user %s: %s", m.Author.Username, err)
	}
	return nil
}

func (control *Control) handleStartServerCommand(member *discordgo.Member, s *discordgo.Session, m *discordgo.Message) error {
	if !memberHasRole(member, control.Config.DiscordAdminRoleID, control.Config.DiscordPowerUserRoleID) {
		return ErrUnauthorized
	}
	var req StartServerRequest
	contentSplit := strings.Split(strings.ToLower(m.Content), " ")
	switch len(contentSplit) {
	case 3:
		req.ServerName = contentSplit[2]
		req.TTL = "12h"
	case 4:
		req.ServerName = contentSplit[2]
		req.TTL = contentSplit[3]
	default:
		return ErrIllegalArguments
	}
	server, err := control.startServer(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to start server for bot: %s", err)
	}
	_, err = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf(
		"Server %s started with DNS %s. It will run for %s",
		server.Name,
		server.PublicNet.IPv4.DNSPtr,
		req.TTL,
	))
	if err != nil {
		return fmt.Errorf("discord: failed to reply to user %s: %s", m.Author.Username, err)
	}
	return nil
}

func (control *Control) handleNewServerCommand(member *discordgo.Member, s *discordgo.Session, m *discordgo.Message) error {
	if !memberHasRole(member, control.Config.DiscordAdminRoleID) {
		return ErrUnauthorized
	}
	var req CreateNewServerRequest
	contentSplit := strings.Split(strings.ToLower(m.Content), " ")
	switch len(contentSplit) {
	case 3:
		req.ServerName = contentSplit[2]
		req.ServerType = "cx11"
		req.TTL = "12h"
	case 4:
		req.ServerName = contentSplit[2]
		req.ServerType = contentSplit[3]
		req.TTL = "12h"
	case 5:
		req.ServerName = contentSplit[2]
		req.ServerType = contentSplit[3]
		req.TTL = contentSplit[4]
	default:
		return ErrIllegalArguments
	}
	server, err := control.newServer(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to create new server for bot: %s", err)
	}
	_, err = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf(
		"Created new server %s with DNS %s. It will run for %s",
		server.Name,
		server.Name+".svc.mnbr.eu",
		req.TTL,
	))
	if err != nil {
		return fmt.Errorf("discord: failed to reply to user %s: %s", m.Author.Username, err)
	}
	return nil
}

func (control *Control) handleExtendServerCommand(member *discordgo.Member, s *discordgo.Session, m *discordgo.Message) error {
	if !memberHasRole(member, control.Config.DiscordAdminRoleID, control.Config.DiscordPowerUserRoleID) {
		return ErrUnauthorized
	}
	contentSplit := strings.Split(strings.ToLower(m.Content), " ")
	if len(contentSplit) != 4 {
		return ErrIllegalArguments
	}
	req := ExtendServerRequest{
		ServerName: contentSplit[2],
		TTL:        contentSplit[3],
		Inverse:    false,
	}
	extendedTTL, err := control.extendServer(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to extend server for bot: %s", err)
	}
	_, err = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf(
		"Server %s has been extended until %s",
		req.ServerName,
		extendedTTL.Format(time.RFC3339),
	))
	if err != nil {
		return fmt.Errorf("discord: failed to reply to user %s: %s", m.Author.Username, err)
	}
	return nil
}

func (control *Control) handlePruneServerCommand(member *discordgo.Member, s *discordgo.Session, m *discordgo.Message) error {
	if !memberHasRole(member, control.Config.DiscordAdminRoleID, control.Config.DiscordPowerUserRoleID) {
		return ErrUnauthorized
	}
	contentSplit := strings.Split(strings.ToLower(m.Content), " ")
	if len(contentSplit) != 4 {
		return ErrIllegalArguments
	}
	req := ExtendServerRequest{
		ServerName: contentSplit[2],
		TTL:        contentSplit[3],
		Inverse:    true,
	}
	extendedTTL, err := control.extendServer(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to prune server for bot: %s", err)
	}
	_, err = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf(
		"Server %s has been pruned to %s",
		req.ServerName,
		extendedTTL.Format(time.RFC3339),
	))
	if err != nil {
		return fmt.Errorf("discord: failed to reply to user %s: %s", m.Author.Username, err)
	}
	return nil
}

func (control *Control) handleRebootServerCommand(member *discordgo.Member, s *discordgo.Session, m *discordgo.Message) error {
	if !memberHasRole(member, control.Config.DiscordAdminRoleID, control.Config.DiscordPowerUserRoleID) {
		return ErrUnauthorized
	}
	contentSplit := strings.Split(strings.ToLower(m.Content), " ")
	if len(contentSplit) != 3 {
		return ErrIllegalArguments
	}
	_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf(
		"Server %s will be rebooted, this might take a while",
		contentSplit[2],
	))
	if err != nil {
		return fmt.Errorf("discord: failed to reply to user %s: %s", m.Author.Username, err)
	}
	err = control.rebootServer(context.Background(), contentSplit[2])
	if err != nil {
		return fmt.Errorf("failed to reboot server for bot: %s", err)
	}
	_, err = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf(
		"Server %s has been rebooted",
		contentSplit[2],
	))
	if err != nil {
		return fmt.Errorf("discord: failed to reply to user %s: %s", m.Author.Username, err)
	}
	return nil
}

func (control *Control) handleTerminateServerCommand(member *discordgo.Member, s *discordgo.Session, m *discordgo.Message) error {
	if !memberHasRole(member, control.Config.DiscordAdminRoleID, control.Config.DiscordPowerUserRoleID) {
		return ErrUnauthorized
	}
	contentSplit := strings.Split(strings.ToLower(m.Content), " ")
	if len(contentSplit) != 3 {
		return ErrIllegalArguments
	}
	_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf(
		"Server %s will be terminated, this might take a while",
		contentSplit[2],
	))
	if err != nil {
		return fmt.Errorf("discord: failed to reply to user %s: %s", m.Author.Username, err)
	}
	err = control.terminateServer(context.Background(), contentSplit[2])
	if err != nil {
		return fmt.Errorf("failed to terminate server for bot: %s", err)
	}
	_, err = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf(
		"Server %s has been terminated",
		contentSplit[2],
	))
	if err != nil {
		return fmt.Errorf("discord: failed to reply to user %s: %s", m.Author.Username, err)
	}
	return nil
}

func (control *Control) handleChangeServerTypeCommand(member *discordgo.Member, s *discordgo.Session, m *discordgo.Message) error {
	if !memberHasRole(member, control.Config.DiscordAdminRoleID) {
		return ErrUnauthorized
	}
	contentSplit := strings.Split(strings.ToLower(m.Content), " ")
	if len(contentSplit) != 4 {
		return ErrIllegalArguments
	}
	req := ChangeServerTypeRequest{
		ServerName: contentSplit[2],
		ServerType: contentSplit[3],
	}
	err := control.changeServerType(context.Background(), req)
	if err != nil {
		return fmt.Errorf("failed to change server type for bot: %s", err)
	}
	_, err = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf(
		"Server %s is now of type %s",
		req.ServerName,
		req.ServerType,
	))
	if err != nil {
		return fmt.Errorf("discord: failed to reply to user %s: %s", m.Author.Username, err)
	}
	return nil
}

func memberHasRole(member *discordgo.Member, roles ...string) bool {
	for _, givenRole := range roles {
		for _, r := range member.Roles {
			if r == givenRole {
				return true
			}
		}
	}
	return false
}

func isPrivateChannel(s *discordgo.Session, channelID string) bool {
	channel, err := s.Channel(channelID)
	if err != nil || channel.Type != discordgo.ChannelTypeDM {
		return false
	}
	return true
}
