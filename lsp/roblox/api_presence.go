package roblox

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var presenceAPIBaseURL = "https://presence.roblox.com"

type PresenceSnapshot struct {
	UserID           int64
	UserPresenceType int
	PlaceID          int64
	RootPlaceID      int64
	UniverseID       int64
	GameID           string
	LastLocation     string
}

func (p PresenceSnapshot) InGame() bool {
	return p.UserPresenceType == 2
}

func (p PresenceSnapshot) state() *PresenceState {
	return &PresenceState{
		UserID:           formatUserID(p.UserID),
		UserPresenceType: p.UserPresenceType,
		PlaceID:          p.PlaceID,
		RootPlaceID:      p.RootPlaceID,
		UniverseID:       p.UniverseID,
		GameID:           p.GameID,
		LastLocation:     p.LastLocation,
		UpdatedAt:        time.Now().Unix(),
	}
}

type presenceRequest struct {
	UserIDs []int64 `json:"userIds"`
}

type presenceResponse struct {
	UserPresences []struct {
		UserID           int64  `json:"userId"`
		UserPresenceType int    `json:"userPresenceType"`
		PlaceID          *int64 `json:"placeId"`
		RootPlaceID      *int64 `json:"rootPlaceId"`
		UniverseID       *int64 `json:"universeId"`
		GameID           string `json:"gameId"`
		LastLocation     string `json:"lastLocation"`
	} `json:"userPresences"`
}

func (c *robloxAPIClient) fetchPresence(ctx context.Context, cfg ProviderConfig, userIDs []int64) (map[string]PresenceSnapshot, error) {
	if len(userIDs) == 0 {
		return map[string]PresenceSnapshot{}, nil
	}
	url := fmt.Sprintf("%s/v1/presence/users", presenceAPIBaseURL)
	req := presenceRequest{UserIDs: userIDs}
	var resp presenceResponse
	withCookie := strings.TrimSpace(cfg.Roblosecurity) != ""
	if _, err := c.doJSON(ctx, cfg, http.MethodPost, url, req, &resp, withCookie, withCookie); err != nil {
		return nil, err
	}

	result := make(map[string]PresenceSnapshot, len(resp.UserPresences))
	for _, item := range resp.UserPresences {
		snapshot := PresenceSnapshot{
			UserID:           item.UserID,
			UserPresenceType: item.UserPresenceType,
			PlaceID:          int64Value(item.PlaceID),
			RootPlaceID:      int64Value(item.RootPlaceID),
			UniverseID:       int64Value(item.UniverseID),
			GameID:           strings.TrimSpace(item.GameID),
			LastLocation:     strings.TrimSpace(item.LastLocation),
		}
		result[strconv.FormatInt(item.UserID, 10)] = snapshot
	}
	return result, nil
}

func int64Value(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}
