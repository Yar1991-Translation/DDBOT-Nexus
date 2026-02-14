package roblox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cnxysoft/DDBOT-WSa/internal/test"
	"github.com/stretchr/testify/require"
)

func TestAddAndRemoveWithUserIDOrUsername(t *testing.T) {
	test.InitBuntdb(t)
	defer test.CloseBuntdb(t)

	usersSrv := newUsersAPIServer(t)
	defer usersSrv.Close()
	restoreUsers := withUsersAPIBaseURL(usersSrv.URL)
	defer restoreUsers()

	c := NewConcern(nil)
	c.FreshIndex(test.G1, test.G2)

	idInfo, err := c.Add(nil, test.G1, "123", Join)
	require.NoError(t, err)
	require.Equal(t, "123", fmt.Sprint(idInfo.GetUid()))

	idInfo, err = c.Add(nil, test.G2, "builderman", Join)
	require.NoError(t, err)
	require.Equal(t, "123", fmt.Sprint(idInfo.GetUid()))

	_, err = c.Add(nil, test.G2, "missing-user", Join)
	require.Error(t, err)

	_, err = c.Remove(nil, test.G1, "builderman", Join)
	require.NoError(t, err)
}

func TestPresenceStateTransitions(t *testing.T) {
	test.InitBuntdb(t)
	defer test.CloseBuntdb(t)
	restoreGameMeta := withGameMetadataAPIServer(t)
	defer restoreGameMeta()

	c := NewConcern(nil)
	require.NoError(t, c.StateManager.SetUserProfile(&UserProfile{
		ID:          123,
		Name:        "builderman",
		DisplayName: "Builderman",
	}))

	cfg := loadProviderConfig()
	cfg.JoinAPIEnabled = false
	cfg.EmitLeave = true

	userID := "123"
	ctx := context.Background()
	enterSnapshot := PresenceSnapshot{
		UserID:           123,
		UserPresenceType: 2,
		PlaceID:          111,
		RootPlaceID:      222,
		UniverseID:       333,
		GameID:           "game-a",
	}
	switchSnapshot := PresenceSnapshot{
		UserID:           123,
		UserPresenceType: 2,
		PlaceID:          111,
		RootPlaceID:      222,
		UniverseID:       333,
		GameID:           "game-b",
	}
	outSnapshot := PresenceSnapshot{
		UserID:           123,
		UserPresenceType: 0,
	}

	events, err := c.handlePresence(ctx, cfg, userID, enterSnapshot)
	require.NoError(t, err)
	require.Len(t, events, 1)
	enterEvent := events[0].(*JoinEvent)
	require.Equal(t, EventEnter, enterEvent.EventKind)
	require.Equal(t, "测试游戏-333", enterEvent.GameName)
	require.Equal(t, "https://img.example.com/icon-333.png", enterEvent.BannerURL)

	events, err = c.handlePresence(ctx, cfg, userID, enterSnapshot)
	require.NoError(t, err)
	require.Len(t, events, 0)

	events, err = c.handlePresence(ctx, cfg, userID, switchSnapshot)
	require.NoError(t, err)
	require.Len(t, events, 1)
	switchEvent := events[0].(*JoinEvent)
	require.Equal(t, EventEnter, switchEvent.EventKind)
	require.Equal(t, "game-a", switchEvent.PrevGameID)
	require.Equal(t, "game-b", switchEvent.GameID)

	events, err = c.handlePresence(ctx, cfg, userID, outSnapshot)
	require.NoError(t, err)
	require.Len(t, events, 1)
	leaveEvent := events[0].(*JoinEvent)
	require.Equal(t, EventLeave, leaveEvent.EventKind)
}

func TestPresenceStateTransitions_EnterWithoutGameID(t *testing.T) {
	test.InitBuntdb(t)
	defer test.CloseBuntdb(t)
	restoreGameMeta := withGameMetadataAPIServer(t)
	defer restoreGameMeta()

	c := NewConcern(nil)
	require.NoError(t, c.StateManager.SetUserProfile(&UserProfile{
		ID:          123,
		Name:        "builderman",
		DisplayName: "Builderman",
	}))

	cfg := loadProviderConfig()
	cfg.JoinAPIEnabled = false
	cfg.EmitLeave = true

	userID := "123"
	ctx := context.Background()
	enterSnapshot := PresenceSnapshot{
		UserID:           123,
		UserPresenceType: 2,
		PlaceID:          111,
		RootPlaceID:      222,
		UniverseID:       333,
		GameID:           "",
	}
	outSnapshot := PresenceSnapshot{
		UserID:           123,
		UserPresenceType: 0,
	}

	events, err := c.handlePresence(ctx, cfg, userID, enterSnapshot)
	require.NoError(t, err)
	require.Len(t, events, 1)
	enterEvent := events[0].(*JoinEvent)
	require.Equal(t, EventEnter, enterEvent.EventKind)
	require.Equal(t, "", enterEvent.GameID)

	events, err = c.handlePresence(ctx, cfg, userID, outSnapshot)
	require.NoError(t, err)
	require.Len(t, events, 1)
	leaveEvent := events[0].(*JoinEvent)
	require.Equal(t, EventLeave, leaveEvent.EventKind)
}

func TestPresenceStateTransitions_GameIDAppearsAfterEnter(t *testing.T) {
	test.InitBuntdb(t)
	defer test.CloseBuntdb(t)
	restoreGameMeta := withGameMetadataAPIServer(t)
	defer restoreGameMeta()

	c := NewConcern(nil)
	require.NoError(t, c.StateManager.SetUserProfile(&UserProfile{
		ID:          123,
		Name:        "builderman",
		DisplayName: "Builderman",
	}))

	cfg := loadProviderConfig()
	cfg.JoinAPIEnabled = false
	cfg.EmitLeave = true

	userID := "123"
	ctx := context.Background()
	enterNoGame := PresenceSnapshot{
		UserID:           123,
		UserPresenceType: 2,
		PlaceID:          111,
		RootPlaceID:      222,
		UniverseID:       333,
		GameID:           "",
	}
	enterWithGame := PresenceSnapshot{
		UserID:           123,
		UserPresenceType: 2,
		PlaceID:          111,
		RootPlaceID:      222,
		UniverseID:       333,
		GameID:           "game-a",
	}

	events, err := c.handlePresence(ctx, cfg, userID, enterNoGame)
	require.NoError(t, err)
	require.Len(t, events, 1)

	events, err = c.handlePresence(ctx, cfg, userID, enterWithGame)
	require.NoError(t, err)
	require.Len(t, events, 1)
	switchEvent := events[0].(*JoinEvent)
	require.Equal(t, EventEnter, switchEvent.EventKind)
	require.Equal(t, "", switchEvent.PrevGameID)
	require.Equal(t, "game-a", switchEvent.GameID)
	require.Equal(t, "测试游戏-333", switchEvent.GameName)
	require.Equal(t, "https://img.example.com/icon-333.png", switchEvent.BannerURL)
}

func TestBuildEnterEventIncludesGameMetadata(t *testing.T) {
	test.InitBuntdb(t)
	defer test.CloseBuntdb(t)
	restoreGameMeta := withGameMetadataAPIServer(t)
	defer restoreGameMeta()

	c := NewConcern(nil)
	require.NoError(t, c.StateManager.SetUserProfile(&UserProfile{
		ID:          123,
		Name:        "builderman",
		DisplayName: "Builderman",
	}))

	cfg := loadProviderConfig()
	cfg.JoinAPIEnabled = false
	cfg.EmitLeave = true

	event, err := c.buildEnterEvent(context.Background(), cfg, "123", PresenceSnapshot{
		UserID:           123,
		UserPresenceType: 2,
		PlaceID:          111,
		RootPlaceID:      222,
		UniverseID:       333,
		GameID:           "game-x",
	}, "")
	require.NoError(t, err)
	require.Equal(t, "测试游戏-333", event.GameName)
	require.Equal(t, "测试地点-222", event.PlaceName)
	require.Equal(t, "https://img.example.com/icon-333.png", event.BannerURL)
}

func TestBuildJoinURLGameJoinAndFallback(t *testing.T) {
	client := newRobloxAPIClient()
	snapshot := PresenceSnapshot{
		UserID:           123,
		UserPresenceType: 2,
		PlaceID:          10,
		RootPlaceID:      20,
		GameID:           "instance-id-1",
	}
	cfg := loadProviderConfig()
	cfg.JoinAPIEnabled = true
	cfg.Roblosecurity = "cookie-for-test"
	cfg.UserAgent = "unit-test"

	t.Run("gamejoin_success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Contains(t, r.Header.Get("Cookie"), ".ROBLOSECURITY=")
			_, _ = w.Write([]byte(`{"joinScriptUrl":"https://join.example.com/path"}`))
		}))
		defer server.Close()

		restore := withGameJoinAPIURL(server.URL)
		defer restore()

		joinURL, source, err := client.buildJoinURL(context.Background(), cfg, snapshot)
		require.NoError(t, err)
		require.Equal(t, "gamejoin", source)
		require.Equal(t, "https://join.example.com/path", joinURL)
	})

	t.Run("gamejoin_fallback_to_deeplink", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"errors":[{"code":1,"message":"boom"}]}`))
		}))
		defer server.Close()

		restore := withGameJoinAPIURL(server.URL)
		defer restore()

		joinURL, source, err := client.buildJoinURL(context.Background(), cfg, snapshot)
		require.NoError(t, err)
		require.Equal(t, "deeplink", source)
		require.Contains(t, joinURL, "roblox.com/games/start")
		require.Contains(t, joinURL, "gameInstanceId=instance-id-1")
	})
}

func newUsersAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/users/"):
			userID := strings.TrimPrefix(r.URL.Path, "/v1/users/")
			if userID == "123" {
				_, _ = w.Write([]byte(`{"id":123,"name":"builderman","displayName":"Builderman"}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"errors":[{"code":1,"message":"not found"}]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/usernames/users":
			var body struct {
				Usernames []string `json:"usernames"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			if len(body.Usernames) == 0 {
				_, _ = w.Write([]byte(`{"data":[]}`))
				return
			}
			switch strings.ToLower(body.Usernames[0]) {
			case "builderman":
				_, _ = w.Write([]byte(`{"data":[{"id":123,"name":"builderman","displayName":"Builderman"}]}`))
			default:
				_, _ = w.Write([]byte(`{"data":[]}`))
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func withGameMetadataAPIServer(t *testing.T) func() {
	t.Helper()

	gamesSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/games":
			universeID := r.URL.Query().Get("universeIds")
			if universeID == "" {
				_, _ = w.Write([]byte(`{"data":[]}`))
				return
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":[{"id":%s,"rootPlaceId":222,"name":"测试游戏-%s"}]}`, universeID, universeID)))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/v1/games/multiget-place-details":
			placeID := r.URL.Query().Get("placeIds")
			if placeID == "" {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`[{"placeId":%s,"name":"测试地点-%s","universeId":333,"universeRootPlaceId":222}]`, placeID, placeID)))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	thumbnailsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/games/multiget/thumbnails":
			universeID := r.URL.Query().Get("universeIds")
			if universeID == "" {
				_, _ = w.Write([]byte(`{"data":[]}`))
				return
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":[{"universeId":%s,"thumbnails":[{"state":"Completed","imageUrl":"https://img.example.com/banner-%s.png"}]}]}`, universeID, universeID)))
			return
		case r.Method == http.MethodGet && r.URL.Path == "/v1/games/icons":
			universeID := r.URL.Query().Get("universeIds")
			if universeID == "" {
				_, _ = w.Write([]byte(`{"data":[]}`))
				return
			}
			_, _ = w.Write([]byte(fmt.Sprintf(`{"data":[{"state":"Completed","imageUrl":"https://img.example.com/icon-%s.png"}]}`, universeID)))
			return
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))

	restoreGames := withGamesAPIBaseURL(gamesSrv.URL)
	restoreThumbnails := withThumbnailsAPIBaseURL(thumbnailsSrv.URL)
	return func() {
		restoreThumbnails()
		restoreGames()
		thumbnailsSrv.Close()
		gamesSrv.Close()
	}
}

func withUsersAPIBaseURL(url string) func() {
	old := usersAPIBaseURL
	usersAPIBaseURL = url
	return func() {
		usersAPIBaseURL = old
	}
}

func withGameJoinAPIURL(url string) func() {
	old := gameJoinAPIURL
	gameJoinAPIURL = url
	return func() {
		gameJoinAPIURL = old
	}
}

func withPresenceAPIBaseURL(url string) func() {
	old := presenceAPIBaseURL
	presenceAPIBaseURL = url
	return func() {
		presenceAPIBaseURL = old
	}
}

func withGamesAPIBaseURL(url string) func() {
	old := gamesAPIBaseURL
	gamesAPIBaseURL = url
	return func() {
		gamesAPIBaseURL = old
	}
}

func withThumbnailsAPIBaseURL(url string) func() {
	old := thumbnailsAPIBaseURL
	thumbnailsAPIBaseURL = url
	return func() {
		thumbnailsAPIBaseURL = old
	}
}

func TestFetchPresenceBatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Contains(t, r.Header.Get("Cookie"), ".ROBLOSECURITY=cookie-for-test")
		_, _ = w.Write([]byte(`{"userPresences":[{"userId":123,"userPresenceType":2,"placeId":11,"rootPlaceId":22,"universeId":33,"gameId":"g-1","lastLocation":"Testing Experience"}]}`))
	}))
	defer server.Close()

	restore := withPresenceAPIBaseURL(server.URL)
	defer restore()

	client := newRobloxAPIClient()
	cfg := loadProviderConfig()
	cfg.Roblosecurity = "cookie-for-test"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := client.fetchPresence(ctx, cfg, []int64{123})
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, "g-1", result["123"].GameID)
	require.Equal(t, "Testing Experience", result["123"].LastLocation)
}
