package radiusserver

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/maronato/authifi/internal/config"
	"github.com/maronato/authifi/internal/database"
	"github.com/maronato/authifi/internal/logging"
	"github.com/maronato/authifi/internal/telegram"
	"golang.org/x/sync/errgroup"
	"layeh.com/radius"
	"layeh.com/radius/rfc2865"
	"layeh.com/radius/rfc2868"
)

const (
	vlanTunnelType rfc2868.TunnelType = 13
	emptyPassword                     = "<empty>"
	filledPassword                    = "********"
)

// setPacketVLAN sets the VLAN information in the RADIUS packet.
func setPacketVLAN(packet *radius.Packet, vlan database.VLAN) {
	rfc2868.TunnelPrivateGroupID_SetString(packet, 0, vlan.ID) //nolint:errcheck // this doesn't return an error

	// Set tunnel type and medium type, defaulting to VLAN(13) and IEEE802(6)
	if vlan.TunnelType != 0 {
		rfc2868.TunnelType_Set(packet, 0, rfc2868.TunnelType(vlan.TunnelType)) //nolint:errcheck // this doesn't return an error
	} else {
		rfc2868.TunnelType_Set(packet, 0, vlanTunnelType) //nolint:errcheck // this doesn't return an error
	}

	if vlan.TunnelMediumType != 0 {
		rfc2868.TunnelMediumType_Set(packet, 0, rfc2868.TunnelMediumType(vlan.TunnelMediumType)) //nolint:errcheck // this doesn't return an error
	} else {
		rfc2868.TunnelMediumType_Set(packet, 0, rfc2868.TunnelMediumType_Value_IEEE802) //nolint:errcheck // this doesn't return an error
	}
}

// StartServer starts the RADIUS server.
func StartServer(ctx context.Context, cfg *config.Config, db database.Database, botServer *telegram.BotServer) error {
	eg, egCtx := errgroup.WithContext(ctx)

	// RADIUS handler for all requests
	handler := radius.HandlerFunc(func(w radius.ResponseWriter, r *radius.Request) {
		startTime := time.Now()

		// Initialize logger and context
		l := logging.FromCtx(egCtx)
		r = r.WithContext(egCtx)

		// Get the request information
		username := rfc2865.UserName_GetString(r.Packet)
		password := rfc2865.UserPassword_GetString(r.Packet)
		macAddress := rfc2865.CallingStationID_GetString(r.Packet)

		// Censor the password and secret in the logs
		privacyPassword := emptyPassword
		if password != "" {
			privacyPassword = filledPassword
		}

		privacySecret := emptyPassword
		if r.Secret != nil {
			privacySecret = filledPassword
		}

		// Create log groups depending on the verbosity level
		var requestGroup slog.Attr
		if cfg.Verbose >= config.VerboseLevelAccessLogs {
			requestGroup = slog.Group("request",
				slog.String("username", username),
				slog.String("password", privacyPassword),
				slog.String("mac_address", macAddress),
				slog.String("remote_addr", r.RemoteAddr.String()),
				slog.String("identifier", fmt.Sprintf("%d", r.Identifier)),
				slog.String("authenticator", fmt.Sprintf("%x", r.Authenticator)),
				slog.String("secret", privacySecret),
				slog.String("code", r.Code.String()),
			)
		} else {
			requestGroup = slog.Group("request",
				slog.String("username", username),
				slog.String("mac_address", macAddress),
				slog.String("remote_addr", r.RemoteAddr.String()),
			)
		}

		// Add the request log group to the logger
		l = l.With(requestGroup)

		// Response packet
		var response *radius.Packet

		// Get default VLAN
		vlan, err := db.GetDefaultVLAN()
		if err != nil {
			l.Debug("error getting default VLAN", slog.Any("error", err))

			// If there's an error getting the default VLAN, default to rejecting the request
			response = r.Response(radius.CodeAccessReject)
		} else {
			// If there's a default VLAN, default to accepting the request and setting the VLAN in the response
			response = r.Response(radius.CodeAccessAccept)
			setPacketVLAN(response, vlan)
		}

		var user database.User

		// Start by checking if the user is blocked
		userBlocked, err := db.IsUserBlocked(username)
		if err != nil { //nolint:nestif // This is the simplest way to handle the errors
			// If there's an error checking if the user is blocked, log it and fallback to rejecting the request
			l.Debug("error checking if user is blocked", slog.Any("error", err))

			response = r.Response(radius.CodeAccessReject)
		} else if userBlocked {
			// If the user is blocked, reject the request
			l.Debug("user is blocked")

			response = r.Response(radius.CodeAccessReject)
		} else if user, err = db.GetUser(username); err != nil {
			// If the user doesn't exist, notify the bot of the login attempt and keep the response as is
			l.Debug("error getting user", slog.Any("error", err))

			// Notify the user of the login attempt
			botServer.NotifyLoginAttempt(username, password, macAddress)
		} else if user.Password != password {
			// If the password is incorrect, reject the request
			l.Debug("incorrect password for user")

			// If the password is incorrect, reject the request
			response = r.Response(radius.CodeAccessReject)
		} else if vlan, err = db.GetVLAN(user.VlanID); err != nil {
			// If there's an error getting the user's VLAN, log it and keep the response as is
			l.Debug("error getting VLAN for user", slog.Any("error", err))
		} else {
			// If the user exists and the password is correct, accept the request and set the VLAN in the response
			response = r.Response(radius.CodeAccessAccept)
			setPacketVLAN(response, vlan)
		}

		// Censor the response secret in the logs
		privacyResponseSecret := emptyPassword
		if response.Secret != nil {
			privacyResponseSecret = filledPassword
		}

		var responseGroup slog.Attr
		// Build response log group depending on the verbosity level
		if cfg.Verbose >= config.VerboseLevelAccessLogs {
			_, rVlanID := rfc2868.TunnelPrivateGroupID_GetString(response)
			_, rTunnelType := rfc2868.TunnelType_Get(response)
			_, rTunnelMediumType := rfc2868.TunnelMediumType_Get(response)

			elapsed := time.Since(startTime)

			responseGroup = slog.Group("response",
				slog.String("code", response.Code.String()),
				slog.String("identifier", fmt.Sprintf("%d", response.Identifier)),
				slog.String("authenticator", fmt.Sprintf("%x", response.Authenticator)),
				slog.String("secret", privacyResponseSecret),
				slog.String("duration", elapsed.String()),
				// VLAN information
				slog.String("vlan_id", rVlanID),
				slog.Any("tunnel_type", rTunnelType),
				slog.Any("tunnel_medium_type", rTunnelMediumType),
			)

			l = l.With(responseGroup)
		}

		// Send the response
		if err := w.Write(response); err != nil {
			l.Error("error sending response", slog.Any("error", err))
		} else if cfg.Verbose >= config.VerboseLevelAccessLogs {
			switch response.Code { //nolint:exhaustive // We only care about these codes
			case radius.CodeAccessAccept:
				l.Info("Access granted")
			case radius.CodeAccessReject:
				l.Info("Access denied")
			default:
				l.Error("Unknown response code")
			}
		}
	})

	l := logging.FromCtx(egCtx)

	// Create the RADIUS server
	server := radius.PacketServer{
		Handler:      handler,
		SecretSource: radius.StaticSecretSource([]byte(cfg.RadiusSecret)),
		Addr:         cfg.GetAddr(),
		ErrorLog:     logging.AsStdLogger(l),
	}

	// Start the server
	eg.Go(func() error {
		l.Info("Starting RADIUS server")

		if err := server.ListenAndServe(); err != nil {
			return fmt.Errorf("error running server: %w", err)
		}

		return nil
	})

	// Shutdown the server if the context is done
	eg.Go(func() error {
		<-egCtx.Done()
		l.Debug("Shutting down RADIUS server")

		// Disable cancel so we can shutdown gracefully
		noCancelCtx := context.WithoutCancel(egCtx)
		if err := server.Shutdown(noCancelCtx); err != nil {
			return fmt.Errorf("error shutting down server: %w", err)
		}

		return nil
	})

	// Wait for the server to exit and check for errors that
	// are not caused by the context being canceled.
	if err := eg.Wait(); err != nil && ctx.Err() == nil {
		return fmt.Errorf("server exited with error: %w", err)
	}

	return nil
}
