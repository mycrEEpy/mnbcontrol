package main

import (
	"flag"
	"log"
	"strconv"
	"strings"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/sirupsen/logrus"
)

var (
	logLevel         = flag.Int("logLevel", 4, "log level (0-6)")
	logReportCaller  = flag.Bool("logReportCaller", true, "log report caller")
	logFormatterJSON = flag.Bool("logFormatterJson", false, "log formatter json")
	locationName     = flag.String("locationName", "nbg1", "location name")
	networkIDs       = flag.String("networkIDs", "", "comma separated list of network ids")
	sshKeyIDs        = flag.String("sshKeyIDs", "", "comma separated list if ssh key ids")
	dnsZoneID        = flag.String("dnsZoneID", "", "dns zone id")
	discordCallback  = flag.String("discordCallback", "http://localhost:8000/auth/callback?provider=discord", "discord oauth callback url")
	discordGuildID   = flag.String("discordGuildID", "", "discord guild id for authorization")
	discordRoleID    = flag.String("discordRoleID", "", "discord role id for authorization")
)

func init() {
	flag.Parse()

	logrus.SetLevel(logrus.Level(*logLevel))
	logrus.SetReportCaller(*logReportCaller)
	if *logFormatterJSON {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}
	log.SetOutput(logrus.StandardLogger().Out)

	AuthSetup(*discordCallback)
}

func main() {
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

	control, err := NewControl(&ControlConfig{
		location:  &hcloud.Location{Name: *locationName},
		networks:  networks,
		sshKeys:   sshKeys,
		dnsZoneID: *dnsZoneID,
	})
	if err != nil {
		logrus.Fatalf("failed to create control: %w", err)
	}

	err = control.Run()
	if err != nil {
		logrus.Fatalf("control api failed: %s", err)
	}
}
