package _routers

import (
	"net/http"

	"github.com/getsentry/sentry-go"
	"github.com/sirupsen/logrus"
	"github.com/turt2live/matrix-media-repo/api/_apimeta"
	"github.com/turt2live/matrix-media-repo/api/_auth_cache"
	"github.com/turt2live/matrix-media-repo/api/_responses"
	"github.com/turt2live/matrix-media-repo/common"
	"github.com/turt2live/matrix-media-repo/common/config"
	"github.com/turt2live/matrix-media-repo/common/rcontext"
	"github.com/turt2live/matrix-media-repo/matrix"
	"github.com/turt2live/matrix-media-repo/util"
)

type GeneratorWithUserFn = func(r *http.Request, ctx rcontext.RequestContext, user _apimeta.UserInfo) interface{}

func RequireAccessToken(generator GeneratorWithUserFn) GeneratorFn {
	return func(r *http.Request, ctx rcontext.RequestContext) interface{} {
		accessToken := util.GetAccessTokenFromRequest(r)
		if accessToken == "" {
			return &_responses.ErrorResponse{
				Code:         common.ErrCodeMissingToken,
				Message:      "no token provided (required)",
				InternalCode: common.ErrCodeMissingToken,
			}
		}
		if config.Get().SharedSecret.Enabled && accessToken == config.Get().SharedSecret.Token {
			ctx = ctx.LogWithFields(logrus.Fields{"sharedSecretAuth": true})
			return generator(r, ctx, _apimeta.UserInfo{
				UserId:      "@sharedsecret",
				AccessToken: accessToken,
				IsShared:    true,
			})
		}
		appserviceUserId := util.GetAppserviceUserIdFromRequest(r)
		userId, err := _auth_cache.GetUserId(ctx, accessToken, appserviceUserId)
		if err != nil || userId == "" {
			if err == matrix.ErrGuestToken {
				return _responses.GuestAuthFailed()
			}
			if err != nil && err != matrix.ErrInvalidToken {
				sentry.CaptureException(err)
				ctx.Log.Error("Error verifying token: ", err)
				return _responses.InternalServerError("unexpected error validating access token")
			}
			return _responses.AuthFailed()
		}

		ctx = ctx.LogWithFields(logrus.Fields{"authUserId": userId})
		return generator(r, ctx, _apimeta.UserInfo{
			UserId:      userId,
			AccessToken: accessToken,
			IsShared:    false,
		})
	}
}
