package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hetznercloud/hcloud-go/hcloud"
	log "github.com/sirupsen/logrus"
)

const (
	LabelManagedBy            = "mnbr.eu/managed-by"
	LabelValueMangedByControl = "mnbcontrol"
	LabelService              = "mnbr.eu/svc"
	LabelTTL                  = "mnbr.eu/ttl"
	LabelActiveBlueprint      = "mnbr.eu/active-blueprint"
)

type Control struct {
	Config  *ControlConfig
	api     *http.Server
	hclient *hcloud.Client
}

type ControlConfig struct {
	location *hcloud.Location
	networks []*hcloud.Network
	sshKeys  []*hcloud.SSHKey
}

type APIError struct {
	Error error
}

type CreateNewServerRequest struct {
	ServerName string `json:"serverName"`
	ServerType string `json:"serverType"`
	TTL        string `json:"ttl"`
}

type StartServerRequest struct {
	ServerType string `json:"serverType"`
	TTL        string `json:"ttl"`
}

func NewControl(config *ControlConfig) (*Control, error) {
	if config == nil {
		return nil, errors.New("config can not be nil")
	}
	control := &Control{Config: config}

	token, ok := os.LookupEnv("HCLOUD_TOKEN")
	if !ok {
		return nil, errors.New("HCLOUD_TOKEN must be set")
	}
	control.hclient = hcloud.NewClient(hcloud.WithToken(token), hcloud.WithPollInterval(1*time.Second))

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery(), gin.Logger())
	control.api = &http.Server{
		Addr:    ":8000",
		Handler: engine,
	}

	engine.GET("/server", control.ListServers)
	engine.POST("/server", control.NewServer)
	engine.POST("/server/:name/_start", control.StartServer)
	engine.DELETE("/server/:name", control.TerminateServer)

	return control, nil
}

func (control *Control) Run() error {
	log.Infof("control api listening on %s", control.api.Addr)
	return control.api.ListenAndServe()
}

func (control *Control) ListServers(ctx *gin.Context) {
	servers, _, err := control.hclient.Server.List(ctx, hcloud.ServerListOpts{})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("failed to list servers: %s", err),
		})
		return
	}

	managedServers := make([]*hcloud.Server, 0)
	for _, s := range servers {
		if s.Labels[LabelManagedBy] == LabelValueMangedByControl {
			managedServers = append(managedServers, s)
		}
	}

	ctx.AbortWithStatusJSON(http.StatusOK, managedServers)
}

func (control *Control) NewServer(ctx *gin.Context) {
	var req CreateNewServerRequest
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			fmt.Errorf("failed to bind request: %s", err),
		})
		return
	}

	allImages, _, err := control.hclient.Image.List(ctx, hcloud.ImageListOpts{})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("failed to list images: %s", err),
		})
		return
	}
	var blueprintImage *hcloud.Image
	for _, image := range allImages {
		if image.Labels[LabelActiveBlueprint] == "true" {
			blueprintImage = image
			break
		}
	}
	if blueprintImage == nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("unable to find active blueprint image for server %s", req.ServerName),
		})
		return
	}

	ttlDuration, err := time.ParseDuration(req.TTL)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			fmt.Errorf("failed to parse ttl duration: %s", err),
		})
		return
	}
	ttl := time.Now().Add(ttlDuration)
	r, _, err := control.hclient.Server.Create(ctx, hcloud.ServerCreateOpts{
		Name:             req.ServerName,
		ServerType:       &hcloud.ServerType{Name: req.ServerType},
		Image:            blueprintImage,
		Location:         control.Config.location,
		StartAfterCreate: hcloud.Bool(true),
		Labels: map[string]string{
			LabelManagedBy: LabelValueMangedByControl,
			LabelService:   req.ServerName,
			LabelTTL:       strconv.Itoa(int(ttl.Unix())),
		},
		Networks: control.Config.networks,
		SSHKeys:  control.Config.sshKeys,
	})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("failed to create server %s: %s", req.ServerName, err),
		})
		return
	}
	ctx.AbortWithStatusJSON(http.StatusCreated, *r.Server)
}

func (control *Control) StartServer(ctx *gin.Context) {
	serverName, ok := ctx.Params.Get("name")
	if !ok {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			errors.New("missing name parameter"),
		})
		return
	}

	var req StartServerRequest
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			fmt.Errorf("failed to bind request: %s", err),
		})
		return
	}

	allImages, _, err := control.hclient.Image.List(ctx, hcloud.ImageListOpts{})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("failed to list images: %s", err),
		})
		return
	}
	var latestServiceImage *hcloud.Image
	for _, image := range allImages {
		if image.Labels[LabelService] == serverName {
			if latestServiceImage == nil {
				latestServiceImage = image
				continue
			}
			if image.Created.After(latestServiceImage.Created) {
				latestServiceImage = image
				continue
			}
		}
	}
	if latestServiceImage == nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("unable to find previous snapshot for server %s", serverName),
		})
		return
	}

	ttlDuration, err := time.ParseDuration(req.TTL)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			fmt.Errorf("failed to parse ttl duration: %s", err),
		})
		return
	}
	ttl := time.Now().Add(ttlDuration)
	r, _, err := control.hclient.Server.Create(ctx, hcloud.ServerCreateOpts{
		Name:             serverName,
		ServerType:       &hcloud.ServerType{Name: req.ServerType},
		Image:            latestServiceImage,
		Location:         control.Config.location,
		StartAfterCreate: hcloud.Bool(true),
		Labels: map[string]string{
			LabelManagedBy: LabelValueMangedByControl,
			LabelService:   serverName,
			LabelTTL:       strconv.Itoa(int(ttl.Unix())),
		},
		Networks: control.Config.networks,
		SSHKeys:  control.Config.sshKeys,
	})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("failed to create server %s: %s", serverName, err),
		})
		return
	}
	ctx.AbortWithStatusJSON(http.StatusCreated, *r.Server)
}

func (control *Control) TerminateServer(ctx *gin.Context) {
	serverName, ok := ctx.Params.Get("name")
	if !ok {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			errors.New("missing name parameter"),
		})
		return
	}

	server, _, err := control.hclient.Server.Get(ctx, serverName)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("failed to get server %s by name: %s", serverName, err),
		})
		return
	}

	shutdownAction, _, err := control.hclient.Server.Shutdown(ctx, server)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("failed to shutdown server %s: %s", serverName, err),
		})
		return
	}
	progressChan, errChan := control.hclient.Action.WatchProgress(ctx, shutdownAction)
	err = func() error {
		for {
			select {
			case progress := <-progressChan:
				log.Infof("shutdown progress for server %s: %d%%", serverName, progress)
				if progress == 100 {
					log.Infof("shutdown complete for server %s", serverName)
					return nil
				}
			case err := <-errChan:
				if err != nil {
					return fmt.Errorf("failed to shutdown server %s: %s", serverName, err)
				}
			}
		}
	}()
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			err,
		})
		return
	}

	imageResult, _, err := control.hclient.Server.CreateImage(ctx, server, &hcloud.ServerCreateImageOpts{
		Type:        hcloud.ImageTypeSnapshot,
		Description: hcloud.String(fmt.Sprintf("%s/%s", serverName, time.Now().Format(time.RFC3339))),
		Labels: map[string]string{
			LabelManagedBy: LabelValueMangedByControl,
			LabelService:   serverName,
		},
	})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("failed to create snapshot for server %s: %s", serverName, err),
		})
		return
	}

	progressChan, errChan = control.hclient.Action.WatchProgress(ctx, imageResult.Action)
	err = func() error {
		for {
			select {
			case progress := <-progressChan:
				log.Infof("snapshot progress for server %s: %d%%", serverName, progress)
				if progress == 100 {
					log.Infof("snapshot complete for server %s", serverName)
					return nil
				}
			case err := <-errChan:
				if err != nil {
					return fmt.Errorf("failed to snapshot server %s: %s", serverName, err)
				}
			}
		}
	}()
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			err,
		})
		return
	}

	if server.Image.Type == hcloud.ImageTypeSnapshot && server.Image.Labels[LabelActiveBlueprint] != "true" && !server.Image.Protection.Delete {
		_, err := control.hclient.Image.Delete(ctx, server.Image)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
				fmt.Errorf("failed to delete image %s[%d]: %s", server.Image.Name, server.Image.ID, err),
			})
			return
		}
		log.Infof("deleted previous snapshot %s[%d]", server.Image.Name, server.Image.ID)
	} else {
		log.Infof("skipping deletion of snapshot")
	}

	// re-get server and check if it's locked until unlocked
	ticker := time.NewTicker(5 * time.Second)
	timeout := time.NewTimer(60 * time.Second)
	err = func() error {
		for {
			select {
			case <-ticker.C:
				server, _, err = control.hclient.Server.Get(ctx, serverName)
				if err != nil {
					return fmt.Errorf("failed to get server %s by name: %s", serverName, err)
				}
				if !server.Locked {
					return nil
				}
			case <-timeout.C:
				return fmt.Errorf("timed out waiting for unlocked server %s", serverName)
			}
		}
	}()
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			err,
		})
		return
	}

	_, err = control.hclient.Server.Delete(ctx, server)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("failed to delete server %s: %s", serverName, err),
		})
		return
	}
	log.Infof("deleted server %s", serverName)

	ctx.Status(http.StatusOK)
}
