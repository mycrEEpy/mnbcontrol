package control

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type APIError struct {
	Error string `json:"error"`
}

type CreateNewServerRequest struct {
	ServerName string `json:"serverName"`
	ServerType string `json:"serverType"`
	TTL        string `json:"ttl"`
}

type StartServerRequest struct {
	ServerName string `json:"serverName"`
	TTL        string `json:"ttl"`
}

type ExtendServerRequest struct {
	ServerName string `json:"serverName"`
	TTL        string `json:"ttl"`
	Inverse    bool   `json:"inverse"`
}

type ChangeServerTypeRequest struct {
	ServerName string `json:"serverName"`
	ServerType string `json:"serverType"`
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

func (control *Control) NewServer(ctx *gin.Context) {
	var req CreateNewServerRequest
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			fmt.Errorf("failed to bind request: %s", err).Error(),
		})
		return
	}

	server, err := control.newServer(ctx, req)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			fmt.Errorf("failed to create new server: %s", err).Error(),
		})
		return
	}

	ctx.JSON(http.StatusCreated, server)
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
	req.ServerName = serverName
	server, err := control.startServer(ctx, req)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			fmt.Errorf("failed to start server: %s", err).Error(),
		})
		return
	}
	ctx.JSON(http.StatusCreated, server)
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

func (control *Control) RebootServer(ctx *gin.Context) {
	serverName, ok := ctx.Params.Get("name")
	if !ok {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			errors.New("missing name parameter").Error(),
		})
		return
	}
	err := control.rebootServer(ctx, serverName)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			err.Error(),
		})
		return
	}
	ctx.Status(http.StatusOK)
}

func (control *Control) ExtendServer(ctx *gin.Context) {
	serverName, ok := ctx.Params.Get("name")
	if !ok {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			errors.New("missing name parameter").Error(),
		})
		return
	}
	var req ExtendServerRequest
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			fmt.Errorf("failed to bind request: %s", err).Error(),
		})
		return
	}
	req.ServerName = serverName

	newTTL, err := control.extendServer(ctx, req)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("failed extend server %s: %s", serverName, err).Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, struct {
		TTL string `json:"ttl"`
	}{
		newTTL.Format(time.RFC3339),
	})
}

func (control *Control) ChangeServerType(ctx *gin.Context) {
	serverName, ok := ctx.Params.Get("name")
	if !ok {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			errors.New("missing name parameter").Error(),
		})
		return
	}
	var req ChangeServerTypeRequest
	err := ctx.ShouldBindJSON(&req)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusBadRequest, APIError{
			fmt.Errorf("failed to bind request: %s", err).Error(),
		})
		return
	}
	req.ServerName = serverName

	err = control.changeServerType(ctx, req)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("failed extend server %s: %s", serverName, err).Error(),
		})
		return
	}

	ctx.Status(http.StatusOK)
}
