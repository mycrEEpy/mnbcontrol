package control

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/discord"
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

	expiration := 1 * time.Hour

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"userID": user.UserID,
		"exp":    time.Now().Add(expiration).Format(time.RFC3339),
	})

	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SIGNING_KEY")))
	if err != nil {
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, APIError{
			fmt.Errorf("failed to sign token: %s", err).Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, struct {
		WebToken string `json:"webToken"`
	}{
		WebToken: tokenString,
	})
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
		var tokenStr string

		authHeader := ctx.Request.Header.Get("Authorization")

		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, APIError{
				errors.New("unauthorized: missing auth").Error(),
			})
			return
		}

		token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
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

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, APIError{
				errors.New("unauthorized: unexpected token claims").Error(),
			})
			return
		}

		expiration, err := time.Parse(time.RFC3339, fmt.Sprint(claims["exp"]))
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, APIError{
				errors.New("unauthorized: unexpected expiration format").Error(),
			})
			return
		}

		if time.Now().After(expiration) {
			ctx.AbortWithStatusJSON(http.StatusUnauthorized, APIError{
				errors.New("unauthorized: token has been expired").Error(),
			})
			return
		}

		member, err := control.discordSession.GuildMember(control.Config.DiscordGuildID, fmt.Sprint(claims["userID"]))
		if err != nil {
			ctx.AbortWithStatusJSON(http.StatusForbidden, APIError{
				fmt.Errorf("forbidden: no guild membership: %s", err).Error(),
			})
			return
		}

		if !memberHasRole(member, control.Config.DiscordAdminRoleID) {
			ctx.AbortWithStatusJSON(http.StatusForbidden, APIError{
				fmt.Errorf("forbidden: permission check failed: %s", err).Error(),
			})
			return
		}

		ctx.Next()
	}
}
