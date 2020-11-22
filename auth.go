package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/discord"
)

const (
	WebTokenCookieName = "mnbcontrol_webtoken"
)

func AuthSetup(callbackURL string) {
	goth.UseProviders(
		discord.New(
			os.Getenv("DISCORD_KEY"),
			os.Getenv("DISCORD_SECRET"),
			callbackURL,
			discord.ScopeIdentify,
		),
	)
}

func AuthLogin(ctx *gin.Context) {
	// try to get the user without re-authenticating
	if user, err := gothic.CompleteUserAuth(ctx.Writer, ctx.Request); err == nil {
		ctx.JSON(http.StatusOK, user)
		return
	}
	gothic.BeginAuthHandler(ctx.Writer, ctx.Request)
}

func AuthCallback(ctx *gin.Context) {
	user, err := gothic.CompleteUserAuth(ctx.Writer, ctx.Request)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("auth callback failed: %s", err).Error(),
		})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userID": user.UserID,
	})
	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SIGNING_KEY")))
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("failed to sign token: %s", err).Error(),
		})
		return
	}
	ctx.SetCookie(WebTokenCookieName, tokenString, 86400, "/", "", false, true)
	ctx.Status(http.StatusOK)
}

func AuthLogout(ctx *gin.Context) {
	err := gothic.Logout(ctx.Writer, ctx.Request)
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("failed to logout: %s", err).Error(),
		})
		return
	}
	ctx.Redirect(http.StatusTemporaryRedirect, "/")
}

func (control *Control) Authorize() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		authCookie, err := ctx.Request.Cookie(WebTokenCookieName)
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, APIError{
				errors.New("unauthorized: missing auth").Error(),
			})
			return
		}

		token, err := jwt.Parse(authCookie.Value, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return []byte(os.Getenv("JWT_SIGNING_KEY")), nil
		})
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, APIError{
				errors.New("unauthorized: failed to parse token").Error(),
			})
			return
		}
		if !token.Valid {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, APIError{
				errors.New("unauthorized: token is invalid").Error(),
			})
			return
		}
		if err = token.Claims.Valid(); err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, APIError{
				errors.New("unauthorized: token claims are invalid").Error(),
			})
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, APIError{
				errors.New("unauthorized: unexpected token claims").Error(),
			})
			return
		}

		member, err := control.discordSession.GuildMember(*discordGuildID, fmt.Sprint(claims["userID"]))
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusForbidden, APIError{
				fmt.Errorf("forbidden: no guild membership: %s", err).Error(),
			})
			return
		}
		var hasAccess bool
		for _, role := range member.Roles {
			if role == *discordRoleID {
				hasAccess = true
				break
			}
		}
		if !hasAccess {
			ctx.AbortWithStatusJSON(http.StatusForbidden, APIError{
				fmt.Errorf("forbidden: permission check failed: %s", err).Error(),
			})
			return
		}

		ctx.Next()
	}
}
