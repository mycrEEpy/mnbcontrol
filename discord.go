package main

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
	listOnlineServerTemplate  = "Status: %s\nType: %v\nDNS: %s\nIPv4: %s\nTTL: %s\n"
	listOfflineServerTemplate = "Status: %s\nType: %v\n"
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

	member, err := s.GuildMember(*discordGuildID, m.Author.ID)
	if err != nil {
		_, err := s.ChannelMessageSend(m.ChannelID, "You are not allowed to talk to me!")
		if err != nil {
			log.Errorf("discord: failed to reply to user %s: %s", m.Member.User.Username, err)
		}
		return
	}

	switch {
	case m.Content == "!server list":
		err = control.handleListServerCommand(member, s, m.Message)
	case strings.HasPrefix(m.Content, "!server start"):
		err = control.handleStartServerCommand(member, s, m.Message)
	case strings.HasPrefix(m.Content, "!server new"):
		err = control.handleNewServerCommand(member, s, m.Message)
	case strings.HasPrefix(m.Content, "!server extend"):
		err = control.handleExtendServerCommand(member, s, m.Message)
	case strings.HasPrefix(m.Content, "!server terminate"):
		err = control.handleTerminateServerCommand(member, s, m.Message)
	default:
		if !isPrivateChannel(s.State, m.ChannelID) {
			return
		}
		if !memberHasRole(member, *discordUserRoleID) && !memberHasRole(member, *discordAdminRoleID) {
			_, err := s.ChannelMessageSend(m.ChannelID, "You are not allowed to talk to me!")
			if err != nil {
				log.Errorf("discord: failed to reply to user %s: %s", m.Member.User.Username, err)
			}
			return
		}
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

func (control *Control) handleListServerCommand(member *discordgo.Member, s *discordgo.Session, m *discordgo.Message) error {
	if m.ChannelID != *discordChannelID && !isPrivateChannel(s.State, m.ChannelID) {
		return nil
	}
	if !memberHasRole(member, *discordUserRoleID) && !memberHasRole(member, *discordAdminRoleID) {
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
				listOnlineServerTemplate,
				server.Status,
				server.ServerType.Name,
				server.PublicNet.IPv4.DNSPtr,
				server.PublicNet.IPv4.IP.String(),
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
				listOfflineServerTemplate,
				"terminated",
				image.Labels[LabelServerType],
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
	if !isPrivateChannel(s.State, m.ChannelID) {
		return nil
	}
	if !memberHasRole(member, *discordAdminRoleID) {
		return ErrUnauthorized
	}
	var req StartServerRequest
	contentSplit := strings.Split(m.Content, " ")
	switch len(contentSplit) {
	case 2:
		req.ServerName = contentSplit[2]
		req.TTL = "12h"
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
		"Server %s started with %s pointing at %s. It will run for %s",
		server.Name,
		server.PublicNet.IPv4.DNSPtr,
		server.PublicNet.IPv4.IP.String(),
		req.TTL,
	))
	if err != nil {
		return fmt.Errorf("discord: failed to reply to user %s: %s", m.Author.Username, err)
	}
	return nil
}

func (control *Control) handleNewServerCommand(member *discordgo.Member, s *discordgo.Session, m *discordgo.Message) error {
	if !isPrivateChannel(s.State, m.ChannelID) {
		return nil
	}
	if !memberHasRole(member, *discordAdminRoleID) {
		return ErrUnauthorized
	}
	var req CreateNewServerRequest
	contentSplit := strings.Split(m.Content, " ")
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
		"Created new server %s with %s pointing at %s. It will run for %s",
		server.Name,
		server.Name+".svc.mnbr.eu",
		server.PublicNet.IPv4.IP.String(),
		req.TTL,
	))
	if err != nil {
		return fmt.Errorf("discord: failed to reply to user %s: %s", m.Author.Username, err)
	}
	return nil
}

func (control *Control) handleExtendServerCommand(member *discordgo.Member, s *discordgo.Session, m *discordgo.Message) error {
	if !isPrivateChannel(s.State, m.ChannelID) {
		return nil
	}
	if !memberHasRole(member, *discordAdminRoleID) {
		return ErrUnauthorized
	}
	contentSplit := strings.Split(m.Content, " ")
	if len(contentSplit) != 4 {
		return ErrIllegalArguments
	}
	req := ExtendServerRequest{
		ServerName: contentSplit[2],
		TTL:        contentSplit[3],
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

func (control *Control) handleTerminateServerCommand(member *discordgo.Member, s *discordgo.Session, m *discordgo.Message) error {
	if !isPrivateChannel(s.State, m.ChannelID) {
		return nil
	}
	if !memberHasRole(member, *discordAdminRoleID) {
		return ErrUnauthorized
	}
	contentSplit := strings.Split(m.Content, " ")
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

func isPrivateChannel(state *discordgo.State, channelID string) bool {
	for _, privateChannel := range state.PrivateChannels {
		if privateChannel.ID == channelID {
			return true
		}
	}
	return false
}
