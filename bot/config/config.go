package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type Config struct {
	*viper.Viper

	mu sync.RWMutex

	configName string
	configType string
	searchPath []string

	callbackMu sync.RWMutex
	callbacks  []func(in fsnotify.Event)

	watchMu      sync.Mutex
	watcher      *fsnotify.Watcher
	watchingDirs map[string]struct{}

	lastDiagnostics Diagnostics
}

func NewConfig() *Config {
	return &Config{
		Viper:      viper.New(),
		configName: "application",
		configType: "yaml",
		searchPath: []string{"."},
	}
}

// GlobalConfig is the process-wide configuration instance.
var GlobalConfig = NewConfig()

func (c *Config) SetConfigName(in string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.configName = in
	c.Viper.SetConfigName(in)
}

func (c *Config) SetConfigType(in string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.configType = in
	c.Viper.SetConfigType(in)
}

func (c *Config) AddConfigPath(in string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, p := range c.searchPath {
		if p == in {
			return
		}
	}
	c.searchPath = append(c.searchPath, in)
	c.Viper.AddConfigPath(in)
}

func (c *Config) LoadConfigCanonical() (ConfigCanonical, Diagnostics, error) {
	c.mu.RLock()
	searchPath := append([]string{}, c.searchPath...)
	configName := c.configName
	configType := c.configType
	c.mu.RUnlock()
	canonical, diagnostics, _, err := loadCanonicalByPath(searchPath, configName, configType)
	return canonical, diagnostics, err
}

func (c *Config) LastDiagnostics() Diagnostics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastDiagnostics
}

func (c *Config) ReadInConfig() error {
	c.mu.RLock()
	searchPath := append([]string{}, c.searchPath...)
	configName := c.configName
	configType := c.configType
	c.mu.RUnlock()

	canonical, diagnostics, resolved, err := loadCanonicalByPath(searchPath, configName, configType)
	if err != nil {
		return err
	}

	newViper := viper.New()
	for _, p := range searchPath {
		newViper.AddConfigPath(p)
	}
	newViper.SetConfigName(configName)
	newViper.SetConfigType(configType)
	for _, key := range canonical.SortedKeys() {
		newViper.Set(key, canonical.Values[key])
	}

	writeTarget := resolved.PreferredWritePath()
	if writeTarget != "" {
		newViper.SetConfigFile(writeTarget)
	}

	c.mu.Lock()
	c.Viper = newViper
	c.lastDiagnostics = diagnostics
	c.mu.Unlock()

	logDiagnostics(diagnostics)
	return nil
}

func (c *Config) OnConfigChange(run func(in fsnotify.Event)) {
	c.callbackMu.Lock()
	defer c.callbackMu.Unlock()
	c.callbacks = append(c.callbacks, run)
}

func (c *Config) WatchConfig() {
	c.watchMu.Lock()
	defer c.watchMu.Unlock()
	if c.watcher != nil {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logrus.WithError(err).Error("unable to create config watcher")
		return
	}

	dirs := c.watchDirs()
	for _, dir := range dirs {
		if err := watcher.Add(dir); err != nil {
			logrus.WithField("dir", dir).WithError(err).Error("unable to watch config directory")
			continue
		}
	}
	if len(dirs) == 0 {
		_ = watcher.Close()
		logrus.Warn("no valid config directories for watcher")
		return
	}

	c.watcher = watcher
	c.watchingDirs = make(map[string]struct{}, len(dirs))
	for _, dir := range dirs {
		c.watchingDirs[dir] = struct{}{}
	}
	go c.watchLoop()
}

func (c *Config) watchLoop() {
	for {
		c.watchMu.Lock()
		watcher := c.watcher
		c.watchMu.Unlock()
		if watcher == nil {
			return
		}
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if !c.isTargetConfigEvent(event) {
				continue
			}
			if err := c.ReadInConfig(); err != nil {
				logrus.WithField("event", event.String()).WithError(err).Error("reload config failed")
				continue
			}
			c.fireCallbacks(event)
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			logrus.WithError(err).Error("config watcher error")
		}
	}
}

func (c *Config) fireCallbacks(event fsnotify.Event) {
	c.callbackMu.RLock()
	callbacks := append([]func(in fsnotify.Event){}, c.callbacks...)
	c.callbackMu.RUnlock()
	for _, callback := range callbacks {
		if callback != nil {
			callback(event)
		}
	}
}

func (c *Config) watchDirs() []string {
	c.mu.RLock()
	searchPath := append([]string{}, c.searchPath...)
	c.mu.RUnlock()

	var dirs []string
	seen := make(map[string]struct{})
	for _, p := range searchPath {
		if p == "" {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		stat, err := os.Stat(abs)
		if err != nil || !stat.IsDir() {
			continue
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		seen[abs] = struct{}{}
		dirs = append(dirs, abs)
	}
	return dirs
}

func (c *Config) isTargetConfigEvent(event fsnotify.Event) bool {
	c.mu.RLock()
	configName := c.configName
	configType := c.configType
	c.mu.RUnlock()

	targets := map[string]struct{}{
		strings.ToLower(fmt.Sprintf("%s.%s", configName, configType)):    {},
		strings.ToLower(fmt.Sprintf("%s.v2.%s", configName, configType)): {},
	}

	base := strings.ToLower(filepath.Base(event.Name))
	_, ok := targets[base]
	return ok
}

func (c *Config) CloseWatcher() {
	c.watchMu.Lock()
	defer c.watchMu.Unlock()
	if c.watcher != nil {
		_ = c.watcher.Close()
		c.watcher = nil
	}
}

// LoadConfigCanonical loads canonical config and diagnostics from configured files.
func LoadConfigCanonical() (ConfigCanonical, Diagnostics, error) {
	return GlobalConfig.LoadConfigCanonical()
}

// Init initializes GlobalConfig using application.yaml/application.v2.yaml.
func Init() {
	GlobalConfig.SetConfigName("application")
	GlobalConfig.SetConfigType("yaml")
	GlobalConfig.AddConfigPath(".")
	GlobalConfig.AddConfigPath("./config")

	err := GlobalConfig.ReadInConfig()
	if err != nil {
		logrus.WithField("config", "GlobalConfig").WithError(err).Fatal("unable to read global config")
	}
}
