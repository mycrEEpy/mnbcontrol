package main

import (
	"flag"
	"log"
	"strconv"
	"strings"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/sirupsen/logrus"
)

const (
	Version = "v0.8.0"
)

var (
	logLevel         = flag.Int("logLevel", 4, "log level (0-6)")
	logReportCaller  = flag.Bool("logReportCaller", true, "log report caller")
	logFormatterJSON = flag.Bool("logFormatterJson", false, "log formatter json")
	listenAddr       = flag.String("listenAddr", ":8000", "http server listen address")
	enableCookieAuth = flag.Bool("enableCookieAuth", false, "set cookie after login")
	locationName     = flag.String("locationName", "nbg1", "location name")
	networkIDs       = flag.String("networkIDs", "", "comma separated list of network ids")
	sshKeyIDs        = flag.String("sshKeyIDs", "", "comma separated list if ssh key ids")
	dnsZoneID        = flag.String("dnsZoneID", "", "dns zone id, can be empty for disabling dns support")
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
	logrus.Infof("control version: %s", Version)
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
		ListenAddr: *listenAddr,
		Location:   &hcloud.Location{Name: *locationName},
		Networks:   networks,
		SSHKeys:    sshKeys,
		DNSZoneID:  *dnsZoneID,
	})
	if err != nil {
		logrus.Fatalf("failed to create control: %w", err)
	}

	err = control.Run()
	if err != nil {
		logrus.Fatalf("control api failed: %s", err)
	}
}
