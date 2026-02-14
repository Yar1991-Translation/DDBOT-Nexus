package roblox

import (
	"sync"
	"time"

	"github.com/cnxysoft/DDBOT-WSa/lsp/concern_type"
	"github.com/cnxysoft/DDBOT-WSa/lsp/mmsg"
	"github.com/cnxysoft/DDBOT-WSa/lsp/template"
	localutils "github.com/cnxysoft/DDBOT-WSa/utils"
	"github.com/sirupsen/logrus"
)

const notifyTemplateName = "notify.group.roblox.join.tmpl"

type EventKind string

const (
	EventEnter EventKind = "enter"
	EventLeave EventKind = "leave"
)

type UserProfile struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

func (u *UserProfile) GetUid() interface{} {
	if u == nil {
		return ""
	}
	return u.ID
}

func (u *UserProfile) GetName() string {
	if u == nil {
		return ""
	}
	if u.DisplayName != "" {
		return u.DisplayName
	}
	if u.Name != "" {
		return u.Name
	}
	return ""
}

type PresenceState struct {
	UserID           string `json:"user_id"`
	UserPresenceType int    `json:"user_presence_type"`
	PlaceID          int64  `json:"place_id"`
	RootPlaceID      int64  `json:"root_place_id"`
	UniverseID       int64  `json:"universe_id"`
	GameID           string `json:"game_id"`
	LastLocation     string `json:"last_location"`
	UpdatedAt        int64  `json:"updated_at"`
}

func (p *PresenceState) InGame() bool {
	return p != nil && p.UserPresenceType == 2
}

type JoinEvent struct {
	EventKind EventKind `json:"event_kind"`
	UserID    string    `json:"user_id"`

	Username    string `json:"username"`
	DisplayName string `json:"display_name"`

	PlaceID      int64  `json:"place_id"`
	RootPlaceID  int64  `json:"root_place_id"`
	UniverseID   int64  `json:"universe_id"`
	GameID       string `json:"game_id"`
	PrevGameID   string `json:"prev_game_id,omitempty"`
	GameName     string `json:"game_name,omitempty"`
	PlaceName    string `json:"place_name,omitempty"`
	BannerURL    string `json:"banner_url,omitempty"`
	LastLocation string `json:"last_location,omitempty"`

	JoinURL    string    `json:"join_url"`
	LinkSource string    `json:"link_source"`
	OccurredAt time.Time `json:"occurred_at"`

	once     sync.Once
	msgCache *mmsg.MSG
}

func (e *JoinEvent) Site() string {
	return Site
}

func (e *JoinEvent) Type() concern_type.Type {
	return Join
}

func (e *JoinEvent) GetUid() interface{} {
	if e == nil {
		return ""
	}
	return e.UserID
}

func (e *JoinEvent) Logger() *logrus.Entry {
	if e == nil {
		return logger
	}
	return logger.WithFields(logrus.Fields{
		"Site":        Site,
		"Type":        Join,
		"UserID":      e.UserID,
		"Username":    e.Username,
		"Display":     e.DisplayName,
		"EventKind":   e.EventKind,
		"GameName":    e.GameName,
		"GameID":      e.GameID,
		"PlaceID":     e.PlaceID,
		"RootPlaceID": e.RootPlaceID,
	})
}

func (e *JoinEvent) toMessage() *mmsg.MSG {
	e.once.Do(func() {
		data := map[string]interface{}{
			"event_kind":    string(e.EventKind),
			"user_id":       e.UserID,
			"username":      e.Username,
			"display_name":  e.DisplayName,
			"place_id":      e.PlaceID,
			"root_place_id": e.RootPlaceID,
			"universe_id":   e.UniverseID,
			"game_id":       e.GameID,
			"prev_game_id":  e.PrevGameID,
			"game_name":     e.GameName,
			"place_name":    e.PlaceName,
			"thumbnail_url": e.BannerURL,
			"banner_url":    e.BannerURL,
			"last_location": e.LastLocation,
			"join_url":      e.JoinURL,
			"link_source":   e.LinkSource,
			"occurred_at":   e.OccurredAt.Local().Format("2006-01-02 15:04:05"),
		}
		msg, err := template.LoadAndExec(notifyTemplateName, data)
		if err != nil {
			logger.WithError(err).Error("roblox template render failed")
			return
		}
		e.msgCache = msg
	})
	if e.msgCache == nil {
		return mmsg.NewText("[roblox] notify render failed")
	}
	return e.msgCache
}

type JoinNotify struct {
	*JoinEvent
	GroupCode int64 `json:"group_code"`
}

func (n *JoinNotify) GetGroupCode() int64 {
	if n == nil {
		return 0
	}
	return n.GroupCode
}

func (n *JoinNotify) ToMessage() *mmsg.MSG {
	if n == nil || n.JoinEvent == nil {
		return mmsg.NewText("[roblox] empty notify")
	}
	return n.JoinEvent.toMessage()
}

func (n *JoinNotify) Logger() *logrus.Entry {
	if n == nil || n.JoinEvent == nil {
		return logger
	}
	return n.JoinEvent.Logger().WithFields(localutils.GroupLogFields(n.GroupCode))
}

func newJoinNotify(groupCode int64, event *JoinEvent) *JoinNotify {
	if event == nil {
		return nil
	}
	return &JoinNotify{
		JoinEvent: event,
		GroupCode: groupCode,
	}
}
