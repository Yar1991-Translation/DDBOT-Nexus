package roblox

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Sora233/MiraiGo-Template/utils"
	"github.com/cnxysoft/DDBOT-WSa/lsp/concern"
	"github.com/cnxysoft/DDBOT-WSa/lsp/concern_type"
	"github.com/cnxysoft/DDBOT-WSa/lsp/mmsg"
	"github.com/sirupsen/logrus"
	"github.com/tidwall/buntdb"
)

const (
	ConcernName = "roblox-concern"
	Site        = "roblox"
	Join        = concern_type.Type("join")
)

var logger = utils.GetModuleLogger(ConcernName)

type Concern struct {
	*StateManager
	api *robloxAPIClient
}

func NewConcern(notify chan<- concern.Notify) *Concern {
	return &Concern{
		StateManager: newStateManager(notify),
		api:          newRobloxAPIClient(),
	}
}

func (c *Concern) Site() string {
	return Site
}

func (c *Concern) Types() []concern_type.Type {
	return []concern_type.Type{Join}
}

func (c *Concern) ParseId(raw string) (interface{}, error) {
	return normalizeInputID(raw)
}

func (c *Concern) Start() error {
	cfg := loadProviderConfig()
	logger.WithFields(logrus.Fields{
		"enabled":         cfg.Enabled,
		"interval":        cfg.Interval.String(),
		"timeout":         cfg.Timeout.String(),
		"batch_size":      cfg.BatchSize,
		"join_api":        cfg.JoinAPIEnabled,
		"emit_leave":      cfg.EmitLeave,
		"username_ttl":    cfg.UsernameCacheTTL.String(),
		"roblosecurity":   cfg.maskedCookie(),
		"join_api_active": cfg.canUseJoinAPI(),
	}).Info("roblox provider config loaded")

	c.StateManager.UseFreshFunc(c.fresh())
	c.StateManager.UseNotifyGeneratorFunc(c.notifyGenerator())
	return c.StateManager.Start()
}

func (c *Concern) Stop() {
	logger.Trace("stopping roblox concern")
	c.StateManager.Stop()
}

func (c *Concern) GetStateManager() concern.IStateManager {
	return c.StateManager
}

func (c *Concern) Add(_ mmsg.IMsgCtx, groupCode int64, id interface{}, ctype concern_type.Type) (concern.IdentityInfo, error) {
	rawID := fmt.Sprint(id)
	cfg := loadProviderConfig()
	reqCtx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	userID, profile, err := c.canonicalizeID(reqCtx, rawID, cfg)
	if err != nil {
		return nil, fmt.Errorf("resolve roblox user failed: %w", err)
	}
	if err := c.StateManager.CheckGroupConcern(groupCode, userID, ctype); err != nil {
		return nil, err
	}
	if _, err := c.StateManager.AddGroupConcern(groupCode, userID, ctype); err != nil {
		return nil, err
	}
	return identityFromProfile(userID, profile), nil
}

func (c *Concern) Remove(_ mmsg.IMsgCtx, groupCode int64, id interface{}, ctype concern_type.Type) (concern.IdentityInfo, error) {
	rawID := fmt.Sprint(id)
	cfg := loadProviderConfig()
	reqCtx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	userID, profile, err := c.canonicalizeID(reqCtx, rawID, cfg)
	if err != nil {
		normalized, normalizeErr := normalizeInputID(rawID)
		if normalizeErr != nil {
			return nil, err
		}
		if !isNumericID(normalized) {
			return nil, err
		}
		userID = normalized
	}

	identity := identityFromProfile(userID, profile)
	if _, err := c.StateManager.RemoveGroupConcern(groupCode, userID, ctype); err != nil {
		return nil, err
	}
	c.cleanupIfNoConcern(userID)
	return identity, nil
}

func (c *Concern) Get(id interface{}) (concern.IdentityInfo, error) {
	rawID := fmt.Sprint(id)
	cfg := loadProviderConfig()
	reqCtx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	userID, profile, err := c.canonicalizeID(reqCtx, rawID, cfg)
	if err != nil {
		return nil, err
	}
	return identityFromProfile(userID, profile), nil
}

func (c *Concern) notifyGenerator() concern.NotifyGeneratorFunc {
	return func(groupCode int64, event concern.Event) []concern.Notify {
		switch e := event.(type) {
		case *JoinEvent:
			notify := newJoinNotify(groupCode, e)
			if notify == nil {
				return nil
			}
			return []concern.Notify{notify}
		default:
			logger.WithField("event_type", fmt.Sprintf("%T", event)).Warn("unknown roblox event type")
			return nil
		}
	}
}

func (c *Concern) fresh() concern.FreshFunc {
	return func(ctx context.Context, eventChan chan<- concern.Event) {
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				cfg := loadProviderConfig()
				if cfg.Enabled {
					if err := c.pollPresence(ctx, eventChan, cfg); err != nil && !errors.Is(err, context.Canceled) {
						logger.WithError(err).Warn("roblox poll failed")
					}
				}
				timer.Reset(cfg.Interval)
			}
		}
	}
}

func (c *Concern) pollPresence(ctx context.Context, eventChan chan<- concern.Event, cfg ProviderConfig) error {
	started := time.Now()
	userIDs, err := c.listWatchedUserIDs()
	if err != nil {
		return err
	}
	if len(userIDs) == 0 {
		logger.WithField("interval", cfg.Interval.String()).Info("roblox poll skipped: no subscriptions")
		return nil
	}

	batchSize := cfg.BatchSize
	if batchSize <= 0 || batchSize > 100 {
		batchSize = 100
	}
	totalBatches := (len(userIDs) + batchSize - 1) / batchSize
	totalEvents := 0

	logger.WithFields(logrus.Fields{
		"users":       len(userIDs),
		"batch_size":  batchSize,
		"batches":     totalBatches,
		"interval":    cfg.Interval.String(),
		"timeout":     cfg.Timeout.String(),
		"join_api":    cfg.JoinAPIEnabled,
		"emit_leave":  cfg.EmitLeave,
		"query_start": started.Format(time.RFC3339),
	}).Info("roblox poll started")

	for start := 0; start < len(userIDs); start += batchSize {
		end := start + batchSize
		if end > len(userIDs) {
			end = len(userIDs)
		}
		currentIDs := userIDs[start:end]
		numericIDs := make([]int64, 0, len(currentIDs))
		for _, userID := range currentIDs {
			id64, err := strconv.ParseInt(userID, 10, 64)
			if err != nil {
				logger.WithField("user_id", userID).WithError(err).Warn("invalid stored roblox user id")
				continue
			}
			numericIDs = append(numericIDs, id64)
		}
		if len(numericIDs) == 0 {
			continue
		}

		reqCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
		presenceMap, err := c.api.fetchPresence(reqCtx, cfg, numericIDs)
		cancel()
		if err != nil {
			return err
		}

		for _, id64 := range numericIDs {
			userID := formatUserID(id64)
			snapshot, ok := presenceMap[userID]
			if !ok {
				snapshot = PresenceSnapshot{UserID: id64}
			}
			logger.WithFields(logrus.Fields{
				"user_id":            userID,
				"user_presence_type": snapshot.UserPresenceType,
				"in_game":            snapshot.InGame(),
				"game_id":            snapshot.GameID,
				"place_id":           snapshot.PlaceID,
				"root_place_id":      snapshot.RootPlaceID,
				"universe_id":        snapshot.UniverseID,
				"last_location":      snapshot.LastLocation,
			}).Info("roblox presence queried")
			events, err := c.handlePresence(ctx, cfg, userID, snapshot)
			if err != nil {
				logger.WithField("user_id", userID).WithError(err).Warn("handle roblox presence failed")
				continue
			}
			totalEvents += len(events)
			for _, event := range events {
				eventChan <- event
			}
		}
	}

	logger.WithFields(logrus.Fields{
		"users":      len(userIDs),
		"events":     totalEvents,
		"batch_size": batchSize,
		"cost":       time.Since(started).String(),
	}).Info("roblox poll completed")

	return nil
}

func (c *Concern) handlePresence(ctx context.Context, cfg ProviderConfig, userID string, snapshot PresenceSnapshot) ([]concern.Event, error) {
	oldState, err := c.StateManager.GetPresenceState(userID)
	if err != nil && err != buntdb.ErrNotFound {
		return nil, err
	}

	oldIn := oldState != nil && oldState.InGame()
	newIn := snapshot.InGame()
	events := make([]concern.Event, 0, 1)

	switch {
	case !oldIn && newIn:
		event, eventErr := c.buildEnterEvent(ctx, cfg, userID, snapshot, "")
		if eventErr == nil {
			events = append(events, event)
			logger.WithFields(logrus.Fields{
				"user_id":       userID,
				"event_kind":    EventEnter,
				"old_in":        oldIn,
				"new_in":        newIn,
				"old_game_id":   "",
				"new_game_id":   snapshot.GameID,
				"root_place_id": snapshot.RootPlaceID,
				"place_id":      snapshot.PlaceID,
			}).Info("roblox presence transition")
		} else {
			logger.WithField("user_id", userID).WithError(eventErr).Warn("build enter event failed")
		}
	case oldIn && !newIn:
		if cfg.EmitLeave {
			events = append(events, c.buildLeaveEvent(ctx, cfg, userID, oldState))
			logger.WithFields(logrus.Fields{
				"user_id":       userID,
				"event_kind":    EventLeave,
				"old_in":        oldIn,
				"new_in":        newIn,
				"old_game_id":   oldState.GameID,
				"new_game_id":   "",
				"root_place_id": oldState.RootPlaceID,
				"place_id":      oldState.PlaceID,
			}).Info("roblox presence transition")
		}
	case oldIn && newIn && strings.TrimSpace(snapshot.GameID) != "" && oldState.GameID != snapshot.GameID:
		event, eventErr := c.buildEnterEvent(ctx, cfg, userID, snapshot, oldState.GameID)
		if eventErr == nil {
			events = append(events, event)
			logger.WithFields(logrus.Fields{
				"user_id":       userID,
				"event_kind":    EventEnter,
				"old_in":        oldIn,
				"new_in":        newIn,
				"old_game_id":   oldState.GameID,
				"new_game_id":   snapshot.GameID,
				"root_place_id": snapshot.RootPlaceID,
				"place_id":      snapshot.PlaceID,
			}).Info("roblox presence transition")
		} else {
			logger.WithField("user_id", userID).WithError(eventErr).Warn("build switch event failed")
		}
	}

	if err := c.StateManager.SetPresenceState(userID, snapshot.state()); err != nil {
		return events, err
	}
	return events, nil
}

func (c *Concern) buildEnterEvent(ctx context.Context, cfg ProviderConfig, userID string, snapshot PresenceSnapshot, prevGameID string) (*JoinEvent, error) {
	profile, _ := c.getOrFetchUserProfile(ctx, cfg, userID)
	snapshot, meta := c.enrichGameMetadata(ctx, cfg, userID, snapshot)

	joinURL, linkSource, err := c.api.buildJoinURL(ctx, cfg, snapshot)
	if err != nil {
		joinURL = buildDeepLink(snapshot)
		if joinURL != "" {
			linkSource = "deeplink"
		}
	}

	gameName := strings.TrimSpace(meta.GameName)
	if gameName == "" {
		gameName = strings.TrimSpace(snapshot.LastLocation)
	}
	username, displayName := nameFields(userID, profile)
	return &JoinEvent{
		EventKind:    EventEnter,
		UserID:       userID,
		Username:     username,
		DisplayName:  displayName,
		PlaceID:      snapshot.PlaceID,
		RootPlaceID:  snapshot.RootPlaceID,
		UniverseID:   snapshot.UniverseID,
		GameID:       snapshot.GameID,
		PrevGameID:   prevGameID,
		GameName:     gameName,
		PlaceName:    strings.TrimSpace(meta.PlaceName),
		BannerURL:    strings.TrimSpace(meta.BannerURL),
		LastLocation: strings.TrimSpace(snapshot.LastLocation),
		JoinURL:      joinURL,
		LinkSource:   linkSource,
		OccurredAt:   time.Now().UTC(),
	}, nil
}

func (c *Concern) buildLeaveEvent(ctx context.Context, cfg ProviderConfig, userID string, oldState *PresenceState) *JoinEvent {
	profile, _ := c.getOrFetchUserProfile(ctx, cfg, userID)
	snapshot := PresenceSnapshot{
		PlaceID:      oldState.PlaceID,
		RootPlaceID:  oldState.RootPlaceID,
		UniverseID:   oldState.UniverseID,
		GameID:       oldState.GameID,
		LastLocation: oldState.LastLocation,
	}
	if id64, err := strconv.ParseInt(userID, 10, 64); err == nil {
		snapshot.UserID = id64
	}
	snapshot, meta := c.enrichGameMetadata(ctx, cfg, userID, snapshot)
	gameName := strings.TrimSpace(meta.GameName)
	if gameName == "" {
		gameName = strings.TrimSpace(snapshot.LastLocation)
	}

	username, displayName := nameFields(userID, profile)
	return &JoinEvent{
		EventKind:    EventLeave,
		UserID:       userID,
		Username:     username,
		DisplayName:  displayName,
		PlaceID:      snapshot.PlaceID,
		RootPlaceID:  snapshot.RootPlaceID,
		UniverseID:   snapshot.UniverseID,
		GameID:       snapshot.GameID,
		GameName:     gameName,
		PlaceName:    strings.TrimSpace(meta.PlaceName),
		BannerURL:    strings.TrimSpace(meta.BannerURL),
		LastLocation: strings.TrimSpace(snapshot.LastLocation),
		OccurredAt:   time.Now().UTC(),
	}
}

func (c *Concern) enrichGameMetadata(ctx context.Context, cfg ProviderConfig, userID string, snapshot PresenceSnapshot) (PresenceSnapshot, GameMetadata) {
	meta, err := c.api.resolveGameMetadata(ctx, cfg, snapshot)
	if err != nil {
		logger.WithField("user_id", userID).WithError(err).Debug("resolve roblox game metadata failed")
	}
	if snapshot.RootPlaceID <= 0 && snapshot.PlaceID > 0 {
		snapshot.RootPlaceID = snapshot.PlaceID
	}
	if meta.PlaceID > 0 {
		snapshot.PlaceID = meta.PlaceID
	}
	if meta.RootPlaceID > 0 {
		snapshot.RootPlaceID = meta.RootPlaceID
	}
	if meta.UniverseID > 0 {
		snapshot.UniverseID = meta.UniverseID
	}
	return snapshot, meta
}

func (c *Concern) listWatchedUserIDs() ([]string, error) {
	_, ids, types, err := c.StateManager.ListConcernState(func(_ int64, _ interface{}, p concern_type.Type) bool {
		return p.ContainAll(Join)
	})
	if err == buntdb.ErrNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ids, types, err = c.StateManager.GroupTypeById(ids, types)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{}, len(ids))
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		userID := normalizeUserID(fmt.Sprint(id))
		if userID == "" {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		result = append(result, userID)
	}
	return result, nil
}

func (c *Concern) canonicalizeID(ctx context.Context, rawID string, cfg ProviderConfig) (string, *UserProfile, error) {
	normalized, err := normalizeInputID(rawID)
	if err != nil {
		return "", nil, err
	}
	if isNumericID(normalized) {
		profile, err := c.getOrFetchUserProfile(ctx, cfg, normalized)
		if err != nil {
			return "", nil, err
		}
		userID := formatUserID(profile.ID)
		_ = c.StateManager.SetUserProfile(profile)
		_ = c.StateManager.SetUsernameMapping(profile.Name, userID, cfg.UsernameCacheTTL)
		return userID, profile, nil
	}

	if mappedID, err := c.StateManager.GetUsernameMapping(normalized); err == nil && mappedID != "" {
		profile, profileErr := c.getOrFetchUserProfile(ctx, cfg, mappedID)
		if profileErr == nil {
			return mappedID, profile, nil
		}
		logger.WithField("username", normalized).WithError(profileErr).Debug("roblox username cache stale")
	}

	profile, err := c.api.resolveUserByUsername(ctx, cfg, normalized)
	if err != nil {
		return "", nil, err
	}
	userID := formatUserID(profile.ID)
	_ = c.StateManager.SetUserProfile(profile)
	_ = c.StateManager.SetUsernameMapping(normalized, userID, cfg.UsernameCacheTTL)
	if profile.Name != "" && !strings.EqualFold(profile.Name, normalized) {
		_ = c.StateManager.SetUsernameMapping(profile.Name, userID, cfg.UsernameCacheTTL)
	}
	return userID, profile, nil
}

func (c *Concern) getOrFetchUserProfile(ctx context.Context, cfg ProviderConfig, userID string) (*UserProfile, error) {
	userID = normalizeUserID(userID)
	if userID == "" {
		return nil, errEmptyUserID
	}
	if cached, err := c.StateManager.GetUserProfile(userID); err == nil && cached != nil {
		return cached, nil
	}
	id64, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid user id %q", userID)
	}
	profile, err := c.api.fetchUserByID(ctx, cfg, id64)
	if err != nil {
		return nil, err
	}
	_ = c.StateManager.SetUserProfile(profile)
	_ = c.StateManager.SetUsernameMapping(profile.Name, userID, cfg.UsernameCacheTTL)
	return profile, nil
}

func (c *Concern) cleanupIfNoConcern(userID string) {
	allType, err := c.StateManager.GetConcern(userID)
	if err != nil || !allType.Empty() {
		return
	}
	_ = c.StateManager.DeletePresenceState(userID)
	_ = c.StateManager.DeleteUserProfile(userID)
}

func identityFromProfile(userID string, profile *UserProfile) concern.IdentityInfo {
	username, displayName := nameFields(userID, profile)
	if displayName != "" {
		return concern.NewIdentity(userID, displayName)
	}
	return concern.NewIdentity(userID, username)
}

func nameFields(userID string, profile *UserProfile) (username string, displayName string) {
	username = userID
	displayName = userID
	if profile != nil {
		if profile.Name != "" {
			username = profile.Name
		}
		if profile.DisplayName != "" {
			displayName = profile.DisplayName
		} else if profile.Name != "" {
			displayName = profile.Name
		}
	}
	return username, displayName
}

func normalizeInputID(raw string) (string, error) {
	val := strings.TrimSpace(raw)
	val = strings.TrimPrefix(val, "@")
	if val == "" {
		return "", errors.New("empty roblox id")
	}
	return val, nil
}

func normalizeUserID(userID string) string {
	return strings.TrimSpace(userID)
}

func isNumericID(userID string) bool {
	if userID == "" {
		return false
	}
	for _, ch := range userID {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

func formatUserID(id int64) string {
	return strconv.FormatInt(id, 10)
}
