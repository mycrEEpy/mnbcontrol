package main

import (
	"flag"
	"strconv"
	"strings"

	"github.com/hetznercloud/hcloud-go/hcloud"
	log "github.com/sirupsen/logrus"
)

var (
	blueprintImageId = flag.Int("blueprintImageId", 26408426, "blueprint image id")
	locationName     = flag.String("locationName", "nbg1", "location name")
	networkIds       = flag.String("networkIds", "194958", "comma separated list of network ids")
	sshKeyIds        = flag.String("sshKeyIds", "2403353,2355137", "comma separated list if ssh key ids")
)

func main() {
	flag.Parse()
	var networks []*hcloud.Network
	var sshKeys []*hcloud.SSHKey
	networkIdsSplit := strings.Split(*networkIds, ",")
	for _, networkIdStr := range networkIdsSplit {
		networkId, err := strconv.Atoi(networkIdStr)
		if err != nil {
			log.Fatalf("networkIds must be int")
		}
		networks = append(networks, &hcloud.Network{ID: networkId})
	}
	sshKeyIdsSplit := strings.Split(*sshKeyIds, ",")
	for _, sshKeyIdsStr := range sshKeyIdsSplit {
		sshKeyId, err := strconv.Atoi(sshKeyIdsStr)
		if err != nil {
			log.Fatalf("sshKeyIds must be int")
		}
		sshKeys = append(sshKeys, &hcloud.SSHKey{ID: sshKeyId})
	}
	control, err := NewControl(&ControlConfig{
		blueprintImage: &hcloud.Image{ID: *blueprintImageId},
		location:       &hcloud.Location{Name: *locationName},
		networks:       networks,
		sshKeys:        sshKeys,
	})
	if err != nil {
		log.Fatalf("failed to create control: %w", err)
	}

	err = control.Run()
	if err != nil {
		log.Fatalf("control api failed: %s", err)
	}
}
