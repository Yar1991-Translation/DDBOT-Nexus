package roblox

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

var (
	gamesAPIBaseURL      = "https://games.roblox.com"
	thumbnailsAPIBaseURL = "https://thumbnails.roblox.com"
)

type GameMetadata struct {
	UniverseID  int64
	RootPlaceID int64
	PlaceID     int64
	GameName    string
	PlaceName   string
	BannerURL   string
}

type gameDetail struct {
	UniverseID  int64
	RootPlaceID int64
	Name        string
}

type placeDetail struct {
	PlaceID             int64
	UniverseID          int64
	UniverseRootPlaceID int64
	Name                string
}

func (c *robloxAPIClient) resolveGameMetadata(ctx context.Context, cfg ProviderConfig, snapshot PresenceSnapshot) (GameMetadata, error) {
	meta := GameMetadata{
		UniverseID:  snapshot.UniverseID,
		RootPlaceID: snapshot.RootPlaceID,
		PlaceID:     snapshot.PlaceID,
	}
	if meta.RootPlaceID <= 0 && meta.PlaceID > 0 {
		meta.RootPlaceID = meta.PlaceID
	}

	var firstErr error
	recordErr := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if meta.UniverseID > 0 {
		detail, err := c.fetchGameDetailByUniverse(ctx, cfg, meta.UniverseID)
		if err != nil {
			recordErr(err)
		} else if detail != nil {
			if detail.UniverseID > 0 {
				meta.UniverseID = detail.UniverseID
			}
			if detail.RootPlaceID > 0 {
				meta.RootPlaceID = detail.RootPlaceID
			}
			if strings.TrimSpace(detail.Name) != "" {
				meta.GameName = strings.TrimSpace(detail.Name)
			}
		}
	}

	if meta.UniverseID <= 0 || meta.RootPlaceID <= 0 || meta.GameName == "" || meta.PlaceName == "" {
		placeID := meta.RootPlaceID
		if placeID <= 0 {
			placeID = meta.PlaceID
		}
		if placeID > 0 {
			detail, err := c.fetchPlaceDetail(ctx, cfg, placeID)
			if err != nil {
				recordErr(err)
			} else if detail != nil {
				if detail.UniverseID > 0 {
					meta.UniverseID = detail.UniverseID
				}
				if detail.UniverseRootPlaceID > 0 {
					meta.RootPlaceID = detail.UniverseRootPlaceID
				}
				if meta.RootPlaceID <= 0 && detail.PlaceID > 0 {
					meta.RootPlaceID = detail.PlaceID
				}
				if detail.PlaceID > 0 {
					meta.PlaceID = detail.PlaceID
				}
				if strings.TrimSpace(detail.Name) != "" {
					meta.PlaceName = strings.TrimSpace(detail.Name)
					if meta.GameName == "" {
						meta.GameName = meta.PlaceName
					}
				}
			}
		}
	}

	if meta.UniverseID > 0 {
		banner, err := c.fetchGameThumbnail(ctx, cfg, meta.UniverseID)
		if err != nil {
			recordErr(err)
		} else {
			meta.BannerURL = strings.TrimSpace(banner)
		}
	}

	if meta.GameName == "" && snapshot.LastLocation != "" {
		meta.GameName = strings.TrimSpace(snapshot.LastLocation)
	}

	if meta.GameName != "" || meta.BannerURL != "" || meta.UniverseID > 0 || meta.RootPlaceID > 0 {
		return meta, nil
	}
	return meta, firstErr
}

func (c *robloxAPIClient) fetchGameDetailByUniverse(ctx context.Context, cfg ProviderConfig, universeID int64) (*gameDetail, error) {
	if universeID <= 0 {
		return nil, fmt.Errorf("invalid universe id")
	}
	params := url.Values{}
	params.Set("universeIds", fmt.Sprintf("%d", universeID))
	rawURL := fmt.Sprintf("%s/v1/games?%s", gamesAPIBaseURL, params.Encode())

	var resp struct {
		Data []struct {
			ID          int64  `json:"id"`
			RootPlaceID int64  `json:"rootPlaceId"`
			Name        string `json:"name"`
		} `json:"data"`
	}
	if _, err := c.doJSON(ctx, cfg, http.MethodGet, rawURL, nil, &resp, false, false); err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("game detail not found for universe id %d", universeID)
	}
	item := resp.Data[0]
	return &gameDetail{
		UniverseID:  item.ID,
		RootPlaceID: item.RootPlaceID,
		Name:        strings.TrimSpace(item.Name),
	}, nil
}

func (c *robloxAPIClient) fetchPlaceDetail(ctx context.Context, cfg ProviderConfig, placeID int64) (*placeDetail, error) {
	if placeID <= 0 {
		return nil, fmt.Errorf("invalid place id")
	}
	params := url.Values{}
	params.Set("placeIds", fmt.Sprintf("%d", placeID))
	rawURL := fmt.Sprintf("%s/v1/games/multiget-place-details?%s", gamesAPIBaseURL, params.Encode())

	var resp []struct {
		PlaceID             int64  `json:"placeId"`
		UniverseID          int64  `json:"universeId"`
		UniverseRootPlaceID int64  `json:"universeRootPlaceId"`
		Name                string `json:"name"`
	}
	if _, err := c.doJSON(ctx, cfg, http.MethodGet, rawURL, nil, &resp, false, false); err != nil {
		return nil, err
	}
	if len(resp) == 0 {
		return nil, fmt.Errorf("place detail not found for place id %d", placeID)
	}
	item := resp[0]
	return &placeDetail{
		PlaceID:             item.PlaceID,
		UniverseID:          item.UniverseID,
		UniverseRootPlaceID: item.UniverseRootPlaceID,
		Name:                strings.TrimSpace(item.Name),
	}, nil
}

func (c *robloxAPIClient) fetchGameThumbnail(ctx context.Context, cfg ProviderConfig, universeID int64) (string, error) {
	if universeID <= 0 {
		return "", fmt.Errorf("invalid universe id")
	}

	// Use game icon first because it is the canonical thumbnail.
	params := url.Values{}
	params.Set("universeIds", fmt.Sprintf("%d", universeID))
	params.Set("size", "512x512")
	params.Set("format", "Png")
	params.Set("isCircular", "false")
	rawURL := fmt.Sprintf("%s/v1/games/icons?%s", thumbnailsAPIBaseURL, params.Encode())
	var iconResp struct {
		Data []thumbnailItem `json:"data"`
	}
	if _, err := c.doJSON(ctx, cfg, http.MethodGet, rawURL, nil, &iconResp, false, false); err != nil {
		return "", err
	}
	if imageURL := pickImageURL(iconResp.Data); imageURL != "" {
		return imageURL, nil
	}

	// Fallback: universe thumbnails.
	params = url.Values{}
	params.Set("universeIds", fmt.Sprintf("%d", universeID))
	params.Set("countPerUniverse", "1")
	params.Set("defaults", "true")
	params.Set("size", "768x432")
	params.Set("format", "Png")
	params.Set("isCircular", "false")
	rawURL = fmt.Sprintf("%s/v1/games/multiget/thumbnails?%s", thumbnailsAPIBaseURL, params.Encode())

	var resp struct {
		Data []struct {
			UniverseID int64           `json:"universeId"`
			Thumbnails []thumbnailItem `json:"thumbnails"`
		} `json:"data"`
	}
	if _, err := c.doJSON(ctx, cfg, http.MethodGet, rawURL, nil, &resp, false, false); err != nil {
		return "", err
	}
	for _, item := range resp.Data {
		if item.UniverseID != universeID {
			continue
		}
		if imageURL := pickImageURL(item.Thumbnails); imageURL != "" {
			return imageURL, nil
		}
	}
	return "", nil
}

type thumbnailItem struct {
	State    string `json:"state"`
	ImageURL string `json:"imageUrl"`
}

func pickImageURL(items []thumbnailItem) string {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item.State), "Completed") {
			if imageURL := strings.TrimSpace(item.ImageURL); imageURL != "" {
				return imageURL
			}
		}
	}
	for _, item := range items {
		if imageURL := strings.TrimSpace(item.ImageURL); imageURL != "" {
			return imageURL
		}
	}
	return ""
}
