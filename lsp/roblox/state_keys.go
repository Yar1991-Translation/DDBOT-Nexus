package roblox

import (
	"errors"
	"strings"
	"time"

	localdb "github.com/cnxysoft/DDBOT-WSa/lsp/buntdb"
	"github.com/cnxysoft/DDBOT-WSa/lsp/concern"
	"github.com/tidwall/buntdb"
)

var errEmptyUserID = errors.New("empty user id")

type StateManager struct {
	*concern.StateManager
}

func newStateManager(notify chan<- concern.Notify) *StateManager {
	return &StateManager{
		StateManager: concern.NewStateManagerWithStringID(Site, notify),
	}
}

func (s *StateManager) userProfileKey(userID string) string {
	return localdb.NamedKey("RobloxUserProfile", []interface{}{userID})
}

func (s *StateManager) presenceStateKey(userID string) string {
	return localdb.NamedKey("RobloxPresenceState", []interface{}{userID})
}

func (s *StateManager) usernameMapKey(username string) string {
	return localdb.NamedKey("RobloxUsernameMap", []interface{}{strings.ToLower(strings.TrimSpace(username))})
}

func (s *StateManager) SetUserProfile(profile *UserProfile) error {
	if profile == nil || profile.ID <= 0 {
		return errors.New("invalid user profile")
	}
	return s.SetJson(s.userProfileKey(formatUserID(profile.ID)), profile)
}

func (s *StateManager) GetUserProfile(userID string) (*UserProfile, error) {
	userID = normalizeUserID(userID)
	if userID == "" {
		return nil, errEmptyUserID
	}
	var profile *UserProfile
	if err := s.GetJson(s.userProfileKey(userID), &profile); err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, buntdb.ErrNotFound
	}
	return profile, nil
}

func (s *StateManager) DeleteUserProfile(userID string) error {
	_, err := s.Delete(s.userProfileKey(normalizeUserID(userID)), localdb.IgnoreNotFoundOpt())
	return err
}

func (s *StateManager) SetUsernameMapping(username, userID string, ttl time.Duration) error {
	username = strings.ToLower(strings.TrimSpace(username))
	userID = normalizeUserID(userID)
	if username == "" || userID == "" {
		return nil
	}
	if ttl <= 0 {
		return s.Set(s.usernameMapKey(username), userID)
	}
	return s.Set(s.usernameMapKey(username), userID, localdb.SetExpireOpt(ttl))
}

func (s *StateManager) GetUsernameMapping(username string) (string, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		return "", errors.New("empty username")
	}
	return s.Get(s.usernameMapKey(username))
}

func (s *StateManager) SetPresenceState(userID string, state *PresenceState) error {
	userID = normalizeUserID(userID)
	if userID == "" {
		return errEmptyUserID
	}
	if state == nil {
		return s.DeletePresenceState(userID)
	}
	state.UserID = userID
	return s.SetJson(s.presenceStateKey(userID), state)
}

func (s *StateManager) GetPresenceState(userID string) (*PresenceState, error) {
	userID = normalizeUserID(userID)
	if userID == "" {
		return nil, errEmptyUserID
	}
	var state *PresenceState
	if err := s.GetJson(s.presenceStateKey(userID), &state); err != nil {
		return nil, err
	}
	if state == nil {
		return nil, buntdb.ErrNotFound
	}
	return state, nil
}

func (s *StateManager) DeletePresenceState(userID string) error {
	_, err := s.Delete(s.presenceStateKey(normalizeUserID(userID)), localdb.IgnoreNotFoundOpt())
	return err
}
