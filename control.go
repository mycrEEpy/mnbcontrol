package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
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
	LabelDNSRecordID          = "mnbr.eu/dns-record-id"
)

type Control struct {
	Config         *ControlConfig
	api            *http.Server
	hclient        *hcloud.Client
	discordSession *discordgo.Session
}

type ControlConfig struct {
	ListenAddr string
	Location   *hcloud.Location
	Networks   []*hcloud.Network
	SSHKeys    []*hcloud.SSHKey
	DNSZoneID  string
}

type APIError struct {
	Error string `json:"error"`
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

	var err error
	control.discordSession, err = discordgo.New("Bot " + os.Getenv("DISCORD_BOT_TOKEN"))
	if err != nil {
		return nil, fmt.Errorf("failed to create discord session: %s", err)
	}

	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	engine.Use(gin.Recovery(), gin.Logger())

	//store := cookie.NewStore([]byte(os.Getenv("SESSION_SECRET")))
	//engine.Use(sessions.Sessions("mnbcontrol_session", store))
	//engine.Use(csrf.Middleware(csrf.Options{
	//	Secret: os.Getenv("CSRF_SECRET"),
	//	ErrorFunc: func(c *gin.Context) {
	//		c.AbortWithStatusJSON(http.StatusBadRequest, APIError{
	//			errors.New("CSRF token mismatch").Error(),
	//		})
	//	},
	//}))
	//gothic.Store = store

	control.api = &http.Server{
		Addr:    config.ListenAddr,
		Handler: engine,
	}

	apiV1 := engine.Group("/api/v1")
	apiV1.Use(control.Authorize())

	apiServer := apiV1.Group("/server")
	apiServer.GET("/", control.ListServers)
	apiServer.POST("/", control.NewServer)
	apiServer.POST("/:name/_start", control.StartServer)
	apiServer.DELETE("/:name", control.TerminateServer)

	auth := engine.Group("/auth")
	auth.GET("/", AuthLogin)
	auth.GET("/callback", AuthCallback)
	auth.GET("/logout", AuthLogout)

	//engine.Static("/", "./web")

	return control, nil
}

func (control *Control) Run() error {
	log.Info("control is warming up")
	err := control.discordSession.Open()
	if err != nil {
		return fmt.Errorf("failed to open discord session: %s", err)
	}

	shutdownWG := &sync.WaitGroup{}
	shutdownChan := make(chan os.Signal)
	daemonQuitChan := make(chan os.Signal)
	signal.Notify(shutdownChan, syscall.SIGINT, syscall.SIGTERM)
	go control.daemon(daemonQuitChan, shutdownWG)
	go control.waitForShutdown(shutdownChan, daemonQuitChan, shutdownWG)

	log.Infof("control api listening on %s", control.api.Addr)
	if err = control.api.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("control api server failed: %s", err)
	}
	shutdownWG.Wait()
	return nil
}

func (control *Control) daemon(quit <-chan os.Signal, wg *sync.WaitGroup) {
	log.Infof("control daemon started")
	wg.Add(1)
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			log.Debug("daemon ticker triggered")
			managedServers, err := control.listServers(context.Background())
			if err != nil {
				log.Errorf("daemon error: %s", err)
				break
			}
			now := time.Now()
			for _, s := range managedServers {
				log.Debugf("checking ttl for managedServer: %+v", *s)
				if s.Status != hcloud.ServerStatusRunning && s.Status != hcloud.ServerStatusOff {
					log.Infof("daemon warn: server %s is in status %s, skipping", s.Name, s.Status)
					continue
				}
				ttlStr, ok := s.Labels[LabelTTL]
				if !ok {
					log.Errorf("daemon error: ttl label missing on server %s", s.Name)
					continue
				}
				ttlInt, err := strconv.Atoi(ttlStr)
				if err != nil {
					log.Errorf("daemon error: failed to parse ttl: %s", err)
					continue
				}
				ttl := time.Unix(int64(ttlInt), 0)
				if now.After(ttl) {
					log.Infof("daemon: server %s is past its ttl, terminating now", s.Name)
					err = control.terminateServer(context.Background(), s.Name)
					if err != nil {
						log.Errorf("daemon error: failed to terminate server %s: %s", s.Name, err)
						continue
					}
				}
				log.Debugf("duration until server %s will reach its ttl: %s -> %s", s.Name, ttl.Sub(now), ttl)
			}
			ticker.Reset(1 * time.Minute)
		case <-quit:
			wg.Done()
			log.Info("daemon shutdown complete")
			return
		}
	}
}

func (control *Control) waitForShutdown(shutdownChan <-chan os.Signal, quitChan chan<- os.Signal, shutdownWG *sync.WaitGroup) {
	shutdownWG.Add(1)
	sig := <-shutdownChan
	quitChan <- sig
	err := control.discordSession.Close()
	if err != nil {
		log.Errorf("failed to close discord session: %s", err)
	}
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	err = control.api.Shutdown(ctx)
	if err != nil {
		log.Errorf("failed to shutdown api server: %s", err)
	}
	log.Info("control shutdown complete, see you next time!")
	shutdownWG.Done()
}

func (control *Control) ListServers(ctx *gin.Context) {
	managedServers, err := control.listServers(ctx)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			err.Error(),
		})
		return
	}
	ctx.JSON(http.StatusOK, managedServers)
}

func (control *Control) listServers(ctx context.Context) ([]*hcloud.Server, error) {
	servers, _, err := control.hclient.Server.List(ctx, hcloud.ServerListOpts{})
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %s", err)
	}
	var managedServers []*hcloud.Server
	for _, s := range servers {
		if s.Labels[LabelManagedBy] == LabelValueMangedByControl {
			managedServers = append(managedServers, s)
		}
	}
	return managedServers, nil
}

func (control *Control) NewServer(ctx *gin.Context) {
	var req CreateNewServerRequest
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			fmt.Errorf("failed to bind request: %s", err).Error(),
		})
		return
	}

	allImages, _, err := control.hclient.Image.List(ctx, hcloud.ImageListOpts{})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("failed to list images: %s", err).Error(),
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
			fmt.Errorf("unable to find active blueprint image for server %s", req.ServerName).Error(),
		})
		return
	}

	ttlDuration, err := time.ParseDuration(req.TTL)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			fmt.Errorf("failed to parse ttl duration: %s", err).Error(),
		})
		return
	}
	ttl := time.Now().Add(ttlDuration)
	r, _, err := control.hclient.Server.Create(ctx, hcloud.ServerCreateOpts{
		Name:             req.ServerName,
		ServerType:       &hcloud.ServerType{Name: req.ServerType},
		Image:            blueprintImage,
		Location:         control.Config.Location,
		StartAfterCreate: hcloud.Bool(true),
		Labels: map[string]string{
			LabelManagedBy: LabelValueMangedByControl,
			LabelService:   req.ServerName,
			LabelTTL:       strconv.Itoa(int(ttl.Unix())),
		},
		Networks: control.Config.Networks,
		SSHKeys:  control.Config.SSHKeys,
	})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("failed to create server %s: %s", req.ServerName, err).Error(),
		})
		return
	}

	err = control.attachDNSRecordToServer(ctx, r.Server)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("failed to attach dns record to server %s: %s", req.ServerName, err).Error(),
		})
		return
	}

	ctx.JSON(http.StatusCreated, *r.Server)
}

func (control *Control) StartServer(ctx *gin.Context) {
	serverName, ok := ctx.Params.Get("name")
	if !ok {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			errors.New("missing name parameter").Error(),
		})
		return
	}

	var req StartServerRequest
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			fmt.Errorf("failed to bind request: %s", err).Error(),
		})
		return
	}

	allImages, _, err := control.hclient.Image.List(ctx, hcloud.ImageListOpts{})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("failed to list images: %s", err).Error(),
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
			fmt.Errorf("unable to find previous snapshot for server %s", serverName).Error(),
		})
		return
	}

	ttlDuration, err := time.ParseDuration(req.TTL)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			fmt.Errorf("failed to parse ttl duration: %s", err).Error(),
		})
		return
	}
	ttl := time.Now().Add(ttlDuration)
	r, _, err := control.hclient.Server.Create(ctx, hcloud.ServerCreateOpts{
		Name:             serverName,
		ServerType:       &hcloud.ServerType{Name: req.ServerType},
		Image:            latestServiceImage,
		Location:         control.Config.Location,
		StartAfterCreate: hcloud.Bool(true),
		Labels: map[string]string{
			LabelManagedBy: LabelValueMangedByControl,
			LabelService:   serverName,
			LabelTTL:       strconv.Itoa(int(ttl.Unix())),
		},
		Networks: control.Config.Networks,
		SSHKeys:  control.Config.SSHKeys,
	})
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("failed to create server %s: %s", serverName, err).Error(),
		})
		return
	}

	if len(control.Config.DNSZoneID) > 0 {
		err = control.attachDNSRecordToServer(ctx, r.Server)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
				fmt.Errorf("failed to attach dns record to server %s: %s", serverName, err).Error(),
			})
			return
		}
	}

	ctx.JSON(http.StatusCreated, *r.Server)
}

func (control *Control) TerminateServer(ctx *gin.Context) {
	serverName, ok := ctx.Params.Get("name")
	if !ok {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			errors.New("missing name parameter").Error(),
		})
		return
	}
	err := control.terminateServer(ctx, serverName)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			err.Error(),
		})
		return
	}
	ctx.Status(http.StatusOK)
}

func (control *Control) terminateServer(ctx context.Context, serverName string) error {
	server, _, err := control.hclient.Server.Get(ctx, serverName)
	if err != nil {
		return fmt.Errorf("failed to get server %s by name: %s", serverName, err)
	}

	shutdownAction, _, err := control.hclient.Server.Shutdown(ctx, server)
	if err != nil {
		return fmt.Errorf("failed to shutdown server %s: %s", serverName, err)
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
		return err
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
		return fmt.Errorf("failed to create snapshot for server %s: %s", serverName, err)
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
		return err
	}

	if server.Image.Type == hcloud.ImageTypeSnapshot && server.Image.Labels[LabelActiveBlueprint] != "true" && !server.Image.Protection.Delete {
		_, err := control.hclient.Image.Delete(ctx, server.Image)
		if err != nil {
			return fmt.Errorf("failed to delete image %s[%d]: %s", server.Image.Name, server.Image.ID, err)
		}
		log.Infof("deleted previous snapshot %s[%d]", server.Image.Name, server.Image.ID)
	} else {
		log.Infof("skipping deletion of snapshot")
	}

	// re-get server and check if it's locked until unlocked
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	timeout := time.NewTimer(60 * time.Second)
	defer timeout.Stop()
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
		return err
	}

	_, err = control.hclient.Server.Delete(ctx, server)
	if err != nil {
		return fmt.Errorf("failed to delete server %s: %s", serverName, err)
	}
	log.Infof("deleted server %s", serverName)

	if recordID, ok := server.Labels[LabelDNSRecordID]; ok {
		err = deleteDNSRecord(recordID)
		if err != nil {
			return fmt.Errorf("failed to delete dns record for server %s: %s", serverName, err)
		}
	}

	return nil
}

func (control *Control) attachDNSRecordToServer(ctx context.Context, server *hcloud.Server) error {
	dnsRecordID, err := createDNSRecord(control.Config.DNSZoneID, server.Name+".svc", server.PublicNet.IPv4.IP.String())
	if err != nil {
		return fmt.Errorf("failed to create dns: %s", err)
	}
	labels := server.Labels
	labels[LabelDNSRecordID] = dnsRecordID
	_, _, err = control.hclient.Server.Update(ctx, server, hcloud.ServerUpdateOpts{Labels: labels})
	if err != nil {
		return fmt.Errorf("failed to attach dns record id to labels: %s", err)
	}
	_, _, err = control.hclient.Server.ChangeDNSPtr(ctx, server, server.PublicNet.IPv4.IP.String(), hcloud.String(server.Name+".svc.mnbr.eu"))
	if err != nil {
		return fmt.Errorf("failed to change reverse dns pointer for server %s: %s", server.Name, err)
	}
	return nil
}
