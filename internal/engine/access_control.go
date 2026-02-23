package engine

import (
	"strings"

	"rfguard/internal/config"
)

type AccessControlSet struct {
	Enabled          bool
	WhitelistOnly    bool
	GlobalWhitelist  map[string]struct{}
	GlobalBlacklist  map[string]struct{}
	ReaderWhitelists map[string]map[string]struct{}
	ReaderBlacklists map[string]map[string]struct{}
}

func buildAccessControl(cfg *config.Config) *AccessControlSet {
	ac := &AccessControlSet{Enabled: cfg.AccessControl.Enabled, WhitelistOnly: cfg.AccessControl.WhitelistOnly}
	if !ac.Enabled {
		return ac
	}
	ac.GlobalWhitelist = buildUIDSet(cfg.AccessControl.Whitelist)
	ac.GlobalBlacklist = buildUIDSet(cfg.AccessControl.Blacklist)
	ac.ReaderWhitelists = buildUIDMap(cfg.AccessControl.ReaderWhitelists)
	ac.ReaderBlacklists = buildUIDMap(cfg.AccessControl.ReaderBlacklists)
	return ac
}

func buildUIDSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(values))
	for _, v := range values {
		uid := normalizeUID(v)
		if uid == "" {
			continue
		}
		set[uid] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

func buildUIDMap(values map[string][]string) map[string]map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]map[string]struct{}, len(values))
	for reader, list := range values {
		set := buildUIDSet(list)
		if len(set) == 0 {
			continue
		}
		out[reader] = set
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (a *AccessControlSet) IsBlacklisted(readerID, uid string) bool {
	if a == nil || uid == "" {
		return false
	}
	if a.GlobalBlacklist != nil {
		if _, ok := a.GlobalBlacklist[uid]; ok {
			return true
		}
	}
	if a.ReaderBlacklists != nil {
		if set, ok := a.ReaderBlacklists[readerID]; ok {
			if _, ok := set[uid]; ok {
				return true
			}
		}
	}
	return false
}

func (a *AccessControlSet) IsWhitelisted(readerID, uid string) bool {
	if a == nil || uid == "" {
		return false
	}
	if a.GlobalWhitelist != nil {
		if _, ok := a.GlobalWhitelist[uid]; ok {
			return true
		}
	}
	if a.ReaderWhitelists != nil {
		if set, ok := a.ReaderWhitelists[readerID]; ok {
			if _, ok := set[uid]; ok {
				return true
			}
		}
	}
	return false
}

func normalizeUID(uid string) string {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(uid))
	for _, r := range uid {
		switch {
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r >= 'A' && r <= 'F':
			b.WriteRune(r)
		case r >= 'a' && r <= 'f':
			b.WriteRune(r - 'a' + 'A')
		}
	}
	return b.String()
}
