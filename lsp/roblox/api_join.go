package roblox

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

var gameJoinAPIURL = "https://gamejoin.roblox.com/v1/join-game-instance"

type joinGameInstanceRequest struct {
	PlaceID           int64  `json:"placeId"`
	GameID            string `json:"gameId"`
	IsTeleport        bool   `json:"isTeleport"`
	GameJoinAttemptID string `json:"gameJoinAttemptId"`
}

func (c *robloxAPIClient) buildJoinURL(ctx context.Context, cfg ProviderConfig, snapshot PresenceSnapshot) (string, string, error) {
	if strings.TrimSpace(snapshot.GameID) == "" {
		return "", "", fmt.Errorf("empty game id")
	}
	if cfg.canUseJoinAPI() {
		joinURL, err := c.callJoinAPI(ctx, cfg, snapshot)
		if err != nil {
			logger.WithError(err).WithField("user_id", snapshot.UserID).Debug("roblox gamejoin failed, fallback to deeplink")
		}
		if joinURL != "" {
			return joinURL, "gamejoin", nil
		}
	}
	deepLink := buildDeepLink(snapshot)
	if deepLink == "" {
		return "", "", fmt.Errorf("failed to build deep link")
	}
	return deepLink, "deeplink", nil
}

func (c *robloxAPIClient) callJoinAPI(ctx context.Context, cfg ProviderConfig, snapshot PresenceSnapshot) (string, error) {
	placeID := snapshot.PlaceID
	if placeID <= 0 {
		placeID = snapshot.RootPlaceID
	}
	if placeID <= 0 {
		return "", fmt.Errorf("empty place id")
	}
	req := joinGameInstanceRequest{
		PlaceID:           placeID,
		GameID:            snapshot.GameID,
		IsTeleport:        false,
		GameJoinAttemptID: uuid.NewString(),
	}

	var raw map[string]interface{}
	if _, err := c.doJSON(ctx, cfg, http.MethodPost, gameJoinAPIURL, req, &raw, true, true); err != nil {
		return "", err
	}

	for _, key := range []string{
		"joinScriptUrl",
		"joinScriptURL",
		"authenticationUrl",
		"authenticationURL",
		"url",
	} {
		if val := normalizeCandidateURL(raw[key]); val != "" {
			return val, nil
		}
	}
	return "", nil
}

func normalizeCandidateURL(v interface{}) string {
	raw := strings.TrimSpace(fmt.Sprint(v))
	if raw == "" || raw == "<nil>" {
		return ""
	}
	if _, err := url.ParseRequestURI(raw); err != nil {
		return ""
	}
	return raw
}

func buildDeepLink(snapshot PresenceSnapshot) string {
	placeID := snapshot.RootPlaceID
	if placeID <= 0 {
		placeID = snapshot.PlaceID
	}
	gameID := strings.TrimSpace(snapshot.GameID)
	if placeID <= 0 || gameID == "" {
		return ""
	}
	values := url.Values{}
	values.Set("placeId", fmt.Sprintf("%d", placeID))
	values.Set("gameInstanceId", gameID)
	return "https://www.roblox.com/games/start?" + values.Encode()
}
