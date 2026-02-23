package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	LogLevel      string              `json:"log_level" yaml:"log_level"`
	Ingest        IngestConfig        `json:"ingest" yaml:"ingest"`
	Detection     DetectionConfig     `json:"detection" yaml:"detection"`
	AccessControl AccessControlConfig `json:"access_control" yaml:"access_control"`
	API           APIConfig           `json:"api" yaml:"api"`
	Storage       StorageConfig       `json:"storage" yaml:"storage"`
	Metrics       MetricsConfig       `json:"metrics" yaml:"metrics"`
	Alerts        AlertsConfig        `json:"alerts" yaml:"alerts"`
}

type IngestConfig struct {
	ChannelBuffer int             `json:"channel_buffer" yaml:"channel_buffer"`
	REST          RESTConfig      `json:"rest" yaml:"rest"`
	Syslog        SyslogConfig    `json:"syslog" yaml:"syslog"`
	TCPStream     TCPStreamConfig `json:"tcp_stream" yaml:"tcp_stream"`
	FileTail      FileTailConfig  `json:"file_tail" yaml:"file_tail"`
	Kafka         KafkaConfig     `json:"kafka" yaml:"kafka"`
	Parser        ParserConfig    `json:"parser" yaml:"parser"`
}

type RESTConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Addr    string `json:"addr" yaml:"addr"`
}

type SyslogConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	UDPAddr string `json:"udp_addr" yaml:"udp_addr"`
	TCPAddr string `json:"tcp_addr" yaml:"tcp_addr"`
}

type TCPStreamConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Addr    string `json:"addr" yaml:"addr"`
}

type FileTailConfig struct {
	Enabled    bool     `json:"enabled" yaml:"enabled"`
	StartAtEnd bool     `json:"start_at_end" yaml:"start_at_end"`
	Files      []string `json:"files" yaml:"files"`
}

type KafkaConfig struct {
	Enabled bool     `json:"enabled" yaml:"enabled"`
	Brokers []string `json:"brokers" yaml:"brokers"`
	Topic   string   `json:"topic" yaml:"topic"`
	GroupID string   `json:"group_id" yaml:"group_id"`
}

type ParserConfig struct {
	Timezone        string `json:"timezone" yaml:"timezone"`
	DefaultReaderID string `json:"default_reader_id" yaml:"default_reader_id"`
}

type DetectionConfig struct {
	Windows                 []time.Duration `json:"windows" yaml:"windows"`
	APSThreshold            float64         `json:"aps_threshold" yaml:"aps_threshold"`
	FailureRatioThreshold   float64         `json:"failure_ratio_threshold" yaml:"failure_ratio_threshold"`
	UIDDiversityThreshold   float64         `json:"uid_diversity_threshold" yaml:"uid_diversity_threshold"`
	TimingVarianceThreshold float64         `json:"timing_variance_threshold" yaml:"timing_variance_threshold"`
	AttackScoreThreshold    float64         `json:"attack_score_threshold" yaml:"attack_score_threshold"`
	Weights                 WeightsConfig   `json:"weights" yaml:"weights"`
	Epsilon                 float64         `json:"epsilon" yaml:"epsilon"`
	MinAttempts             int             `json:"min_attempts" yaml:"min_attempts"`
	APSElevatedThreshold    float64         `json:"aps_elevated_threshold" yaml:"aps_elevated_threshold"`
	AlertCooldown           time.Duration   `json:"alert_cooldown" yaml:"alert_cooldown"`
	DedupeWindow            time.Duration   `json:"dedupe_window" yaml:"dedupe_window"`
	MaxClockSkew            time.Duration   `json:"max_clock_skew" yaml:"max_clock_skew"`
	MaxFutureSkew           time.Duration   `json:"max_future_skew" yaml:"max_future_skew"`
}

type WeightsConfig struct {
	APS float64 `json:"aps" yaml:"aps"`
	FR  float64 `json:"fr" yaml:"fr"`
	UDS float64 `json:"uds" yaml:"uds"`
	TV  float64 `json:"tv" yaml:"tv"`
}

type AccessControlConfig struct {
	Enabled          bool                `json:"enabled" yaml:"enabled"`
	WhitelistOnly    bool                `json:"whitelist_only" yaml:"whitelist_only"`
	Whitelist        []string            `json:"whitelist" yaml:"whitelist"`
	Blacklist        []string            `json:"blacklist" yaml:"blacklist"`
	ReaderWhitelists map[string][]string `json:"reader_whitelists" yaml:"reader_whitelists"`
	ReaderBlacklists map[string][]string `json:"reader_blacklists" yaml:"reader_blacklists"`
}

type APIConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Addr    string `json:"addr" yaml:"addr"`
}

type StorageConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Driver  string `json:"driver" yaml:"driver"`
	DSN     string `json:"dsn" yaml:"dsn"`
}

type MetricsConfig struct {
	StoreLimit int `json:"store_limit" yaml:"store_limit"`
}

type AlertsConfig struct {
	StoreLimit int `json:"store_limit" yaml:"store_limit"`
}

func DefaultConfig() *Config {
	return &Config{
		LogLevel: "info",
		Ingest: IngestConfig{
			ChannelBuffer: 10000,
			REST:          RESTConfig{Enabled: true, Addr: ":8080"},
			Syslog:        SyslogConfig{Enabled: true, UDPAddr: ":5514", TCPAddr: ":5514"},
			TCPStream:     TCPStreamConfig{Enabled: false, Addr: ":9000"},
			FileTail:      FileTailConfig{Enabled: false, StartAtEnd: true},
			Kafka:         KafkaConfig{Enabled: false},
			Parser:        ParserConfig{Timezone: "UTC", DefaultReaderID: "unknown"},
		},
		Detection: DetectionConfig{
			Windows:                 []time.Duration{1 * time.Second, 10 * time.Second, 60 * time.Second},
			APSThreshold:            20,
			FailureRatioThreshold:   0.7,
			UIDDiversityThreshold:   0.6,
			TimingVarianceThreshold: 0.02,
			AttackScoreThreshold:    100,
			Weights:                 WeightsConfig{APS: 1.0, FR: 50.0, UDS: 40.0, TV: 1.0},
			Epsilon:                 0.0001,
			MinAttempts:             10,
			APSElevatedThreshold:    10,
			AlertCooldown:           5 * time.Second,
			DedupeWindow:            1 * time.Second,
			MaxClockSkew:            2 * time.Second,
			MaxFutureSkew:           2 * time.Second,
		},
		AccessControl: AccessControlConfig{
			Enabled:       false,
			WhitelistOnly: false,
		},
		API:     APIConfig{Enabled: true, Addr: ":8081"},
		Storage: StorageConfig{Enabled: false, Driver: "sqlite", DSN: "file:rfguard.db?_pragma=busy_timeout(5000)"},
		Metrics: MetricsConfig{StoreLimit: 5000},
		Alerts:  AlertsConfig{StoreLimit: 1000},
	}
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	content, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	cfg := DefaultConfig()

	trimmed := strings.TrimSpace(string(content))
	if len(trimmed) == 0 {
		return nil, errors.New("config file is empty")
	}
	var decodeErr error
	if looksLikeJSON(trimmed) {
		decodeErr = json.Unmarshal([]byte(trimmed), cfg)
	} else {
		decodeErr = yaml.Unmarshal([]byte(trimmed), cfg)
	}
	if decodeErr != nil {
		return nil, decodeErr
	}
	applyDefaults(cfg)
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func Save(path string, cfg *Config) error {
	if path == "" || cfg == nil {
		return errors.New("config path or config is empty")
	}
	var data []byte
	var err error
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".json" {
		data, err = json.MarshalIndent(cfg, "", "  ")
	} else {
		data, err = yaml.Marshal(cfg)
	}
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func looksLikeJSON(s string) bool {
	for _, ch := range s {
		if ch == '{' || ch == '[' {
			return true
		}
		if ch > ' ' {
			return false
		}
	}
	return false
}

func applyDefaults(cfg *Config) {
	if len(cfg.Detection.Windows) == 0 {
		cfg.Detection.Windows = []time.Duration{1 * time.Second, 10 * time.Second, 60 * time.Second}
	}
	if cfg.Metrics.StoreLimit <= 0 {
		cfg.Metrics.StoreLimit = 5000
	}
	if cfg.Alerts.StoreLimit <= 0 {
		cfg.Alerts.StoreLimit = 1000
	}
	if cfg.Ingest.ChannelBuffer <= 0 {
		cfg.Ingest.ChannelBuffer = 10000
	}
	if cfg.Ingest.Parser.Timezone == "" {
		cfg.Ingest.Parser.Timezone = "UTC"
	}
	if cfg.Ingest.Parser.DefaultReaderID == "" {
		cfg.Ingest.Parser.DefaultReaderID = "unknown"
	}
	if cfg.Detection.Epsilon <= 0 {
		cfg.Detection.Epsilon = 0.0001
	}
}

func Validate(cfg *Config) error {
	if cfg.API.Enabled && cfg.API.Addr == "" {
		return errors.New("api.addr required when api.enabled is true")
	}
	if cfg.Ingest.REST.Enabled && cfg.Ingest.REST.Addr == "" {
		return errors.New("ingest.rest.addr required when ingest.rest.enabled is true")
	}
	if cfg.Ingest.Syslog.Enabled && cfg.Ingest.Syslog.UDPAddr == "" && cfg.Ingest.Syslog.TCPAddr == "" {
		return errors.New("ingest.syslog.udp_addr or tcp_addr required when ingest.syslog.enabled is true")
	}
	if cfg.Ingest.TCPStream.Enabled && cfg.Ingest.TCPStream.Addr == "" {
		return errors.New("ingest.tcp_stream.addr required when ingest.tcp_stream.enabled is true")
	}
	if cfg.Ingest.FileTail.Enabled && len(cfg.Ingest.FileTail.Files) == 0 {
		return errors.New("ingest.file_tail.files required when ingest.file_tail.enabled is true")
	}
	if cfg.Ingest.Kafka.Enabled {
		if len(cfg.Ingest.Kafka.Brokers) == 0 || cfg.Ingest.Kafka.Topic == "" || cfg.Ingest.Kafka.GroupID == "" {
			return errors.New("ingest.kafka requires brokers, topic, group_id")
		}
	}
	if cfg.Detection.APSThreshold <= 0 {
		return errors.New("detection.aps_threshold must be > 0")
	}
	if cfg.Detection.AttackScoreThreshold <= 0 {
		return errors.New("detection.attack_score_threshold must be > 0")
	}
	for _, win := range cfg.Detection.Windows {
		if win <= 0 {
			return fmt.Errorf("detection.windows contains non-positive duration: %s", win)
		}
	}
	return nil
}

type Manager struct {
	path    string
	cfg     atomic.Value
	modTime time.Time
}

func NewManager(path string) (*Manager, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}
	m := &Manager{path: path}
	m.cfg.Store(cfg)
	info, err := os.Stat(path)
	if err == nil {
		m.modTime = info.ModTime()
	}
	return m, nil
}

func (m *Manager) Get() *Config {
	if v := m.cfg.Load(); v != nil {
		return v.(*Config)
	}
	return DefaultConfig()
}

func (m *Manager) Path() string {
	return m.path
}

func (m *Manager) Reload() (*Config, error) {
	cfg, err := Load(m.path)
	if err != nil {
		return nil, err
	}
	m.cfg.Store(cfg)
	if info, err := os.Stat(m.path); err == nil {
		m.modTime = info.ModTime()
	}
	return cfg, nil
}

func (m *Manager) Update(cfg *Config) error {
	if cfg == nil {
		return errors.New("nil config")
	}
	if err := Save(m.path, cfg); err != nil {
		return err
	}
	m.cfg.Store(cfg)
	if info, err := os.Stat(m.path); err == nil {
		m.modTime = info.ModTime()
	}
	return nil
}

func (m *Manager) NeedsReload() (bool, error) {
	info, err := os.Stat(m.path)
	if err != nil {
		return false, err
	}
	return info.ModTime().After(m.modTime), nil
}

func (m *Manager) Watch(interval time.Duration, onReload func(*Config), onError func(error), stop <-chan struct{}) {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			needs, err := m.NeedsReload()
			if err != nil {
				if onError != nil {
					onError(err)
				}
				continue
			}
			if !needs {
				continue
			}
			cfg, err := m.Reload()
			if err != nil {
				if onError != nil {
					onError(err)
				}
				continue
			}
			if onReload != nil {
				onReload(cfg)
			}
		case <-stop:
			return
		}
	}
}

func ResolvePath(path string) string {
	if path == "" {
		return path
	}
	if filepath.IsAbs(path) {
		return path
	}
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}
	return filepath.Join(cwd, path)
}
