package main

import (
	"flag"
	"log"
	"strconv"
	"strings"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/mycreepy/mnbcontrol/internal/control"
	"github.com/sirupsen/logrus"
)

var (
	version = "develop"
	commit  string
	date    string
)

var (
	logLevel               = flag.Int("logLevel", 4, "log level (0-6)")
	logReportCaller        = flag.Bool("logReportCaller", true, "log report caller")
	logFormatterJSON       = flag.Bool("logFormatterJson", false, "log formatter json")
	listenAddr             = flag.String("listenAddr", ":8000", "http server listen address")
	locationName           = flag.String("locationName", "nbg1", "location name")
	networkIDs             = flag.String("networkIDs", "", "comma separated list of network ids")
	sshKeyIDs              = flag.String("sshKeyIDs", "", "comma separated list if ssh key ids")
	dnsZoneID              = flag.String("dnsZoneID", "", "dns zone id, can be empty for disabling dns support")
	discordCallback        = flag.String("discordCallback", "http://localhost:8000/auth/callback?provider=discord", "discord oauth callback url")
	discordGuildID         = flag.String("discordGuildID", "", "discord guild id for authorization")
	discordChannelID       = flag.String("discordChannelID", "", "discord channel id for user interaction")
	discordAdminRoleID     = flag.String("discordAdminRoleID", "", "discord role id for admin authorization")
	discordUserRoleID      = flag.String("discordUserRoleID", "", "discord role id for user authorization")
	discordPowerUserRoleID = flag.String("discordPowerUserRoleID", "", "discord role id for power user authorization")
)

func init() {
	flag.Parse()

	logrus.SetLevel(logrus.Level(*logLevel))
	logrus.SetReportCaller(*logReportCaller)
	if *logFormatterJSON {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}
	log.SetOutput(logrus.StandardLogger().Out)

	control.AuthSetup(*discordCallback)
}

func main() {
	infoFields := logrus.Fields{
		"version": version,
		"commit":  commit,
		"date":    date,
	}

	logrus.WithFields(infoFields).Info("control is warming up")

	var networks []*hcloud.Network
	var sshKeys []*hcloud.SSHKey

	if len(*networkIDs) > 0 {
		networkIdsSplit := strings.Split(*networkIDs, ",")
		for _, networkIdStr := range networkIdsSplit {
			networkId, err := strconv.Atoi(networkIdStr)
			if err != nil {
				logrus.Fatalf("networkIDs must be int")
			}
			networks = append(networks, &hcloud.Network{ID: networkId})
		}
	}

	if len(*sshKeyIDs) > 0 {
		sshKeyIdsSplit := strings.Split(*sshKeyIDs, ",")
		for _, sshKeyIdsStr := range sshKeyIdsSplit {
			sshKeyId, err := strconv.Atoi(sshKeyIdsStr)
			if err != nil {
				logrus.Fatalf("sshKeyIDs must be int")
			}
			sshKeys = append(sshKeys, &hcloud.SSHKey{ID: sshKeyId})
		}
	}

	ctrl, err := control.New(&control.Config{
		ListenAddr:             *listenAddr,
		Location:               &hcloud.Location{Name: *locationName},
		Networks:               networks,
		SSHKeys:                sshKeys,
		DNSZoneID:              *dnsZoneID,
		DiscordGuildID:         *discordGuildID,
		DiscordChannelID:       *discordChannelID,
		DiscordAdminRoleID:     *discordAdminRoleID,
		DiscordUserRoleID:      *discordUserRoleID,
		DiscordPowerUserRoleID: *discordPowerUserRoleID,
	})
	if err != nil {
		logrus.Fatalf("failed to create control: %w", err)
	}

	err = ctrl.Run()
	if err != nil {
		logrus.Fatalf("control api failed: %s", err)
	}
}
