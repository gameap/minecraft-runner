package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ServerProperties represents Minecraft server.properties
type ServerProperties struct {
	path       string
	properties map[string]string
}

// LoadServerProperties loads server.properties from a directory
func LoadServerProperties(serverDir string) (*ServerProperties, error) {
	path := filepath.Join(serverDir, "server.properties")
	sp := &ServerProperties{
		path:       path,
		properties: make(map[string]string),
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return sp, nil // Return empty properties if file doesn't exist
		}
		return nil, fmt.Errorf("failed to open server.properties: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			sp.properties[parts[0]] = parts[1]
		}
	}

	return sp, scanner.Err()
}

// Get returns a property value
func (sp *ServerProperties) Get(key string) string {
	return sp.properties[key]
}

// Set sets a property value. Values are kept the way they are written to the
// file, so the value is escaped here, once, like java.util.Properties expects.
func (sp *ServerProperties) Set(key, value string) {
	sp.properties[key] = propertyValueEscaper.Replace(value)
}

// propertyValueEscaper protects the characters java.util.Properties treats
// specially in a value. A backslash in an RCON password would otherwise be
// read by the server as the start of an escape sequence and silently dropped.
var propertyValueEscaper = strings.NewReplacer("\\", "\\\\", "\n", "\\n", "\r", "\\r", "\t", "\\t")

// SetIP sets the server IP
func (sp *ServerProperties) SetIP(ip string) {
	sp.Set("server-ip", ip)
}

// SetPort sets the server port
func (sp *ServerProperties) SetPort(port int) {
	sp.Set("server-port", fmt.Sprintf("%d", port))
}

// SetQueryPort enables query and sets the query port
func (sp *ServerProperties) SetQueryPort(port int) {
	sp.Set("enable-query", "true")
	sp.Set("query.port", fmt.Sprintf("%d", port))
}

// SetRcon enables RCON with the specified port and password
func (sp *ServerProperties) SetRcon(port int, password string) {
	sp.Set("enable-rcon", "true")
	sp.Set("rcon.port", fmt.Sprintf("%d", port))
	sp.Set("rcon.password", password)
}

// Save writes the properties back to the file
func (sp *ServerProperties) Save() error {
	// Read original file to preserve comments and order
	var lines []string
	existingKeys := make(map[string]bool)

	originalFile, err := os.Open(sp.path)
	if err == nil {
		scanner := bufio.NewScanner(originalFile)
		for scanner.Scan() {
			line := scanner.Text()
			trimmed := strings.TrimSpace(line)

			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				lines = append(lines, line)
				continue
			}

			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				existingKeys[key] = true
				if value, ok := sp.properties[key]; ok {
					lines = append(lines, fmt.Sprintf("%s=%s", key, value))
				} else {
					lines = append(lines, line)
				}
			} else {
				lines = append(lines, line)
			}
		}
		originalFile.Close()
	}

	// Add new properties that weren't in the original file
	for key, value := range sp.properties {
		if !existingKeys[key] {
			lines = append(lines, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(sp.path), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write to file
	file, err := os.Create(sp.path)
	if err != nil {
		return fmt.Errorf("failed to create server.properties: %w", err)
	}
	defer file.Close()

	for _, line := range lines {
		fmt.Fprintln(file, line)
	}

	return nil
}

// DefaultServerProperties returns the default Minecraft server.properties content
func DefaultServerProperties() map[string]string {
	return map[string]string{
		"enable-jmx-monitoring":             "false",
		"rcon.port":                         "25575",
		"level-seed":                        "",
		"gamemode":                          "survival",
		"enable-command-block":              "false",
		"enable-query":                      "false",
		"generator-settings":                "{}",
		"enforce-secure-profile":            "true",
		"level-name":                        "world",
		"motd":                              "A Minecraft Server",
		"query.port":                        "25565",
		"pvp":                               "true",
		"generate-structures":               "true",
		"max-chained-neighbor-updates":      "1000000",
		"difficulty":                        "easy",
		"network-compression-threshold":     "256",
		"max-tick-time":                     "60000",
		"require-resource-pack":             "false",
		"use-native-transport":              "true",
		"max-players":                       "20",
		"online-mode":                       "true",
		"enable-status":                     "true",
		"allow-flight":                      "false",
		"initial-disabled-packs":            "",
		"broadcast-rcon-to-ops":             "true",
		"view-distance":                     "10",
		"server-ip":                         "",
		"resource-pack-prompt":              "",
		"allow-nether":                      "true",
		"server-port":                       "25565",
		"enable-rcon":                       "false",
		"sync-chunk-writes":                 "true",
		"op-permission-level":               "4",
		"prevent-proxy-connections":         "false",
		"hide-online-players":               "false",
		"resource-pack":                     "",
		"entity-broadcast-range-percentage": "100",
		"simulation-distance":               "10",
		"rcon.password":                     "",
		"player-idle-timeout":               "0",
		"force-gamemode":                    "false",
		"rate-limit":                        "0",
		"hardcore":                          "false",
		"white-list":                        "false",
		"broadcast-console-to-ops":          "true",
		"spawn-npcs":                        "true",
		"spawn-animals":                     "true",
		"log-ips":                           "true",
		"function-permission-level":         "2",
		"initial-enabled-packs":             "vanilla",
		"level-type":                        "minecraft\\:normal",
		"text-filtering-config":             "",
		"spawn-monsters":                    "true",
		"enforce-whitelist":                 "false",
		"spawn-protection":                  "16",
		"resource-pack-sha1":                "",
		"max-world-size":                    "29999984",
	}
}

// CreateDefaultServerProperties creates a default server.properties file if it doesn't exist
func CreateDefaultServerProperties(serverDir string) error {
	path := filepath.Join(serverDir, "server.properties")

	// Check if file already exists
	if _, err := os.Stat(path); err == nil {
		return nil // File exists, nothing to do
	}

	// Ensure directory exists
	if err := os.MkdirAll(serverDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create server.properties: %w", err)
	}
	defer file.Close()

	fmt.Fprintln(file, "#Minecraft server properties")
	fmt.Fprintln(file, "#Generated by mcrun")

	defaults := DefaultServerProperties()
	for key, value := range defaults {
		fmt.Fprintf(file, "%s=%s\n", key, value)
	}

	return nil
}

// CreateEULA creates an eula.txt file accepting the EULA
func CreateEULA(serverDir string) error {
	eulaPath := filepath.Join(serverDir, "eula.txt")

	// Check if already exists and accepted
	if content, err := os.ReadFile(eulaPath); err == nil {
		if strings.Contains(string(content), "eula=true") {
			return nil
		}
	}

	file, err := os.Create(eulaPath)
	if err != nil {
		return fmt.Errorf("failed to create eula.txt: %w", err)
	}
	defer file.Close()

	fmt.Fprintln(file, "# By changing the setting below to TRUE you are indicating your agreement to the EULA")
	fmt.Fprintln(file, "# https://account.mojang.com/documents/minecraft_eula")
	fmt.Fprintln(file, "eula=true")

	return nil
}
