package acquisition

import (
	"encoding/json"
	"regexp"
	"strings"
)

// redactHostPath replaces the host home directory, without matching sibling
// directories or replacing a bare username that might occur in device evidence.
func redactHostPath(text, home string) string {
	if home == "" || home == "/" || home == "\\" {
		return text
	}
	home = strings.TrimRight(home, "/\\")
	for _, prefix := range []string{strings.ReplaceAll(home, "\\", "\\\\"), home, strings.ReplaceAll(home, "\\", "/")} {
		pattern := regexp.QuoteMeta(prefix) + `($|[/\\\s"'])`
		if len(home) > 1 && home[1] == ':' {
			pattern = "(?i)" + pattern
		}
		text = regexp.MustCompile(pattern).ReplaceAllString(text, "[HOST_HOME]$1")
	}
	return text
}

func hostKeyWithoutComment(key string) string {
	fields := strings.Fields(key)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func (a *Acquisition) archiveMetadata() ([]byte, error) {
	data, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	if !a.HostUsernameRedacted {
		return json.MarshalIndent(a, "", " ")
	}
	// Only host-origin metadata is changed; device paths and identifiers stay
	// intact. Work on the serialized copy so live paths remain usable.
	var info map[string]any
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	redactField := func(object map[string]any, key string) {
		if value, ok := object[key].(string); ok {
			object[key] = redactHostPath(value, a.RedactHostHome)
		}
	}
	redactField(info, "storage_path")
	if key, ok := info["adb_host_public_key"].(string); ok {
		info["adb_host_public_key"] = hostKeyWithoutComment(key)
	}
	if collector, ok := info["collector"].(map[string]any); ok {
		if adb, ok := collector["Adb"].(map[string]any); ok {
			redactField(adb, "ExePath")
		}
	}
	if results, ok := info["module_results"].([]any); ok {
		for _, result := range results {
			if result, ok := result.(map[string]any); ok {
				redactField(result, "error")
			}
		}
	}
	return json.MarshalIndent(info, "", " ")
}

func (a *Acquisition) archiveHostKey() string {
	if a.HostUsernameRedacted {
		return hostKeyWithoutComment(a.ADBHostPublicKey)
	}
	return a.ADBHostPublicKey
}

func (a *Acquisition) archiveLog() []byte {
	if a.HostUsernameRedacted {
		return []byte(redactHostPath(a.logBuffer.String(), a.RedactHostHome))
	}
	return a.logBuffer.Bytes()
}
