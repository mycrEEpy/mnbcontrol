package main

import (
	"context"
	"errors"
	"os"

	"github.com/hetznercloud/hcloud-go/hcloud"
	log "github.com/sirupsen/logrus"
)

func newClient() (*hcloud.Client, error) {
	token, ok := os.LookupEnv("HCLOUD_TOKEN")
	if !ok {
		return nil, errors.New("HCLOUD_TOKEN must be set")
	}
	return hcloud.NewClient(hcloud.WithToken(token)), nil
}

func newServer(client *hcloud.Client) (*hcloud.Server, error) {
	r, _, err := client.Server.Create(context.Background(), hcloud.ServerCreateOpts{
		Name:             "test",
		ServerType:       &hcloud.ServerType{Name: "cx11"},
		Image:            &hcloud.Image{ID: 26408426},
		Location:         &hcloud.Location{Name: "nbg1"},
		StartAfterCreate: hcloud.Bool(true),
		Labels:           map[string]string{"managed-by": "mnbcontrol"},
		Networks:         []*hcloud.Network{{ID: 194958}},
		SSHKeys:          []*hcloud.SSHKey{{ID: 2403353}, {ID: 2355137}},
	})
	if err != nil {
		return nil, err
	}
	return r.Server, nil
}

func main() {
	client, err := newClient()
	if err != nil {
		log.Fatalf("failed to create client: %w", err)
	}

	//l, _, err := client.SSHKey.List(context.Background(), hcloud.SSHKeyListOpts{})
	//if err != nil {
	//	log.Fatalf("failed to list: %w", err)
	//}
	//for _, item := range l {
	//	log.Infof("%+v", item)
	//}

	server, err := newServer(client)
	if err != nil {
		log.Fatalf("failed to create server: %w", err)
	}
	log.Infof("server created: %+v", *server)
}
