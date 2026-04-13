package tunnel

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	pluginpkg "github.com/nguyenviet02/server-management-dashboard/internal/plugin"
	"gorm.io/gorm"
)

type Service struct {
	db     *gorm.DB
	logger *slog.Logger
}

func NewService(db *gorm.DB, logger *slog.Logger) *Service {
	return &Service{db: db, logger: logger}
}

func (s *Service) ListTunnels() ([]Tunnel, error) {
	var tunnels []Tunnel
	if err := s.db.Order("name asc").Find(&tunnels).Error; err != nil {
		return nil, err
	}
	return tunnels, nil
}

func (s *Service) GetTunnel(id uint) (*Tunnel, error) {
	var tunnel Tunnel
	if err := s.db.First(&tunnel, id).Error; err != nil {
		return nil, err
	}
	return &tunnel, nil
}

func (s *Service) CreateTunnel(tunnel *Tunnel) (*Tunnel, error) {
	record, err := s.buildTunnelRecord(tunnel, nil)
	if err != nil {
		return nil, err
	}
	if err := s.db.Create(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

func (s *Service) UpdateTunnel(id uint, updates *Tunnel) (*Tunnel, error) {
	current, err := s.GetTunnel(id)
	if err != nil {
		return nil, err
	}
	record, err := s.buildTunnelRecord(updates, current)
	if err != nil {
		return nil, err
	}
	if err := s.db.Save(record).Error; err != nil {
		return nil, err
	}
	return record, nil
}

func (s *Service) DeleteTunnel(id uint) error {
	return s.db.Delete(&Tunnel{}, id).Error
}

func (s *Service) GetTunnelConfig(id uint) (*Tunnel, *TunnelConfig, error) {
	tunnel, err := s.GetTunnel(id)
	if err != nil {
		return nil, nil, err
	}
	config, err := s.readTunnelConfig(tunnel.ConfigPath)
	if err != nil {
		return nil, nil, err
	}
	return tunnel, config, nil
}

func (s *Service) AddIngress(id uint, item IngressEntry, routeDNS bool) (*Tunnel, *TunnelConfig, string, error) {
	tunnel, config, err := s.GetTunnelConfig(id)
	if err != nil {
		return nil, nil, "", err
	}
	item = normalizeIngressEntry(item, len(config.Ingress))
	if err := validateIngressItem(item); err != nil {
		return nil, nil, "", err
	}
	for _, entry := range config.Ingress {
		if entry.Hostname != "" && entry.Hostname == item.Hostname {
			return nil, nil, "", fmt.Errorf("hostname already exists in config")
		}
	}

	dnsCreated := false
	if routeDNS && item.Hostname != "" {
		if err := s.runCloudflared(buildRouteDNSArgs(tunnel, item.Hostname)...); err != nil {
			return nil, nil, "", err
		}
		dnsCreated = true
	}

	nextIngress := make([]IngressEntry, 0, len(config.Ingress)+1)
	for _, entry := range config.Ingress {
		if entry.Service == "http_status:404" && entry.Hostname == "" {
			continue
		}
		nextIngress = append(nextIngress, entry)
	}
	nextIngress = append(nextIngress, item)

	if err := s.writeTunnelConfig(tunnel.ConfigPath, &TunnelConfig{
		TunnelName:      config.TunnelName,
		CredentialsFile: config.CredentialsFile,
		Ingress:         nextIngress,
	}); err != nil {
		if dnsCreated {
			return nil, nil, "", fmt.Errorf("dns created but config update failed: %w", err)
		}
		return nil, nil, "", err
	}

	_, refreshed, err := s.GetTunnelConfig(id)
	if err != nil {
		return nil, nil, "", err
	}
	warning := ""
	if dnsCreated {
		warning = "DNS route created. If file update had failed, DNS rollback might require manual cleanup because cloudflared CLI does not provide a simple automatic delete flow here."
	}
	return tunnel, refreshed, warning, nil
}

func (s *Service) UpdateIngress(id uint, item IngressEntry, routeDNS bool) (*Tunnel, *TunnelConfig, string, error) {
	tunnel, config, err := s.GetTunnelConfig(id)
	if err != nil {
		return nil, nil, "", err
	}
	if err := validateIngressItem(item); err != nil {
		return nil, nil, "", err
	}

	found := false
	hostnameChanged := false
	nextIngress := make([]IngressEntry, 0, len(config.Ingress))
	for _, entry := range config.Ingress {
		if entry.ID == item.ID {
			found = true
			hostnameChanged = entry.Hostname != item.Hostname
			nextIngress = append(nextIngress, normalizeIngressEntry(item, 0))
			continue
		}
		if item.Hostname != "" && entry.Hostname == item.Hostname {
			return nil, nil, "", fmt.Errorf("hostname already exists in config")
		}
		nextIngress = append(nextIngress, entry)
	}
	if !found {
		return nil, nil, "", fmt.Errorf("ingress entry not found")
	}

	warning := ""
	if routeDNS && hostnameChanged && item.Hostname != "" {
		if err := s.runCloudflared(buildRouteDNSArgs(tunnel, item.Hostname)...); err != nil {
			return nil, nil, "", err
		}
		warning = "If the old hostname DNS record should be removed, please clean it up manually in Cloudflare."
	}

	if err := s.writeTunnelConfig(tunnel.ConfigPath, &TunnelConfig{
		TunnelName:      config.TunnelName,
		CredentialsFile: config.CredentialsFile,
		Ingress:         filterFallbackIngress(nextIngress),
	}); err != nil {
		return nil, nil, "", err
	}

	_, refreshed, err := s.GetTunnelConfig(id)
	if err != nil {
		return nil, nil, "", err
	}
	return tunnel, refreshed, warning, nil
}

func (s *Service) DeleteIngress(id uint, entryID string) (*Tunnel, *TunnelConfig, string, error) {
	tunnel, config, err := s.GetTunnelConfig(id)
	if err != nil {
		return nil, nil, "", err
	}
	nextIngress := make([]IngressEntry, 0, len(config.Ingress))
	removed := false
	for _, entry := range config.Ingress {
		if entry.ID == entryID {
			removed = true
			continue
		}
		nextIngress = append(nextIngress, entry)
	}
	if !removed {
		return nil, nil, "", fmt.Errorf("ingress entry not found")
	}
	if err := s.writeTunnelConfig(tunnel.ConfigPath, &TunnelConfig{
		TunnelName:      config.TunnelName,
		CredentialsFile: config.CredentialsFile,
		Ingress:         filterFallbackIngress(nextIngress),
	}); err != nil {
		return nil, nil, "", err
	}
	_, refreshed, err := s.GetTunnelConfig(id)
	if err != nil {
		return nil, nil, "", err
	}
	return tunnel, refreshed, "Config updated. If you also need to remove the DNS record in Cloudflare, please delete it manually there.", nil
}

func (s *Service) RouteDNS(id uint, hostname string) error {
	tunnel, err := s.GetTunnel(id)
	if err != nil {
		return err
	}
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		return fmt.Errorf("hostname is required")
	}
	return s.runCloudflared(buildRouteDNSArgs(tunnel, hostname)...)
}

func (s *Service) ServiceStatus(id uint) (map[string]any, error) {
	tunnel, err := s.GetTunnel(id)
	if err != nil {
		return nil, err
	}
	serviceName := strings.TrimSpace(tunnel.ServiceName)
	if serviceName == "" {
		return nil, fmt.Errorf("service name is required")
	}
	status := map[string]any{
		"ok":           false,
		"service_name": serviceName,
		"active":       "unknown",
		"enabled":      "unknown",
	}
	active, err := exec.Command("systemctl", "is-active", serviceName).CombinedOutput()
	if err == nil {
		status["ok"] = true
		status["active"] = strings.TrimSpace(string(active))
	} else {
		status["error"] = strings.TrimSpace(string(active))
	}
	enabled, err := exec.Command("systemctl", "is-enabled", serviceName).CombinedOutput()
	if err == nil {
		status["enabled"] = strings.TrimSpace(string(enabled))
	}
	return status, nil
}

func (s *Service) RestartService(id uint) error {
	tunnel, err := s.GetTunnel(id)
	if err != nil {
		return err
	}
	serviceName := strings.TrimSpace(tunnel.ServiceName)
	if serviceName == "" {
		return fmt.Errorf("service name is required")
	}
	cmd := exec.Command("sudo", "systemctl", "restart", serviceName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("restart %s: %s: %s", serviceName, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (s *Service) buildTunnelRecord(input *Tunnel, current *Tunnel) (*Tunnel, error) {
	if input == nil {
		return nil, fmt.Errorf("tunnel payload is required")
	}
	name := strings.TrimSpace(input.Name)
	serviceName := strings.TrimSpace(input.ServiceName)
	configPath, err := ensureFilePath(input.ConfigPath, "config")
	if err != nil {
		return nil, err
	}
	credentialPath, err := ensureFilePath(input.CredentialPath, "credential")
	if err != nil {
		return nil, err
	}
	config, err := s.readTunnelConfig(configPath)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(config.TunnelName) == "" {
		return nil, fmt.Errorf("tunnel name could not be determined from the config file")
	}
	if err := s.ensureUniqueServiceName(serviceName, current); err != nil {
		return nil, err
	}

	record := &Tunnel{
		Name:                name,
		TunnelName:          strings.TrimSpace(config.TunnelName),
		ConfigPath:          configPath,
		CredentialPath:      credentialPath,
		SharedCredentialKey: strings.TrimSpace(input.SharedCredentialKey),
		ServiceName:         serviceName,
	}
	if current != nil {
		record.ID = current.ID
		record.CreatedAt = current.CreatedAt
	}
	return record, nil
}

func (s *Service) ensureUniqueServiceName(serviceName string, current *Tunnel) error {
	if serviceName == "" {
		return nil
	}
	query := s.db.Model(&Tunnel{}).Where("service_name = ?", serviceName)
	if current != nil {
		query = query.Where("id <> ?", current.ID)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("service name already in use by another tunnel")
	}
	return nil
}

func (s *Service) readTunnelConfig(configPath string) (*TunnelConfig, error) {
	resolved, err := ensureFilePath(configPath, "config")
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(resolved)
	if err != nil {
		return nil, err
	}
	parsed := parseConfig(string(content))
	parsed.Path = resolved
	return parsed, nil
}

func (s *Service) writeTunnelConfig(configPath string, config *TunnelConfig) error {
	resolved := filepath.Clean(configPath)
	existing, err := os.ReadFile(resolved)
	if err != nil {
		return err
	}
	backupPath := resolved + ".bak"
	if err := os.WriteFile(backupPath, existing, 0644); err != nil {
		return err
	}
	nextContent := serializeConfig(config)
	if err := os.WriteFile(resolved, []byte(nextContent), 0644); err != nil {
		_ = os.WriteFile(resolved, existing, 0644)
		return err
	}
	return nil
}

func (s *Service) runCloudflared(args ...string) error {
	cmd := exec.Command("cloudflared", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("cloudflared failed: %s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func ensureFilePath(filePath, label string) (string, error) {
	if strings.TrimSpace(filePath) == "" {
		return "", fmt.Errorf("%s path is required", label)
	}
	resolved := filepath.Clean(filePath)
	stat, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("%s path is invalid: %w", label, err)
	}
	if stat.IsDir() {
		return "", fmt.Errorf("%s path must point to a file", label)
	}
	return resolved, nil
}

func parseConfig(content string) *TunnelConfig {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	config := &TunnelConfig{}
	inIngress := false
	var current map[string]string
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "tunnel:"):
			config.TunnelName = strings.TrimSpace(strings.TrimPrefix(line, "tunnel:"))
		case strings.HasPrefix(line, "credentials-file:"):
			config.CredentialsFile = strings.TrimSpace(strings.TrimPrefix(line, "credentials-file:"))
		case line == "ingress:":
			inIngress = true
		case inIngress && strings.HasPrefix(line, "- "):
			if current != nil {
				config.Ingress = append(config.Ingress, normalizeIngressMap(current, len(config.Ingress)))
			}
			current = map[string]string{}
			remainder := strings.TrimSpace(strings.TrimPrefix(line, "- "))
			if remainder != "" && strings.Contains(remainder, ":") {
				parts := strings.SplitN(remainder, ":", 2)
				current[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		case inIngress && current != nil && strings.Contains(line, ":"):
			parts := strings.SplitN(line, ":", 2)
			current[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	if current != nil {
		config.Ingress = append(config.Ingress, normalizeIngressMap(current, len(config.Ingress)))
	}
	return config
}

func serializeConfig(config *TunnelConfig) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("tunnel: %s", config.TunnelName))
	if config.CredentialsFile != "" {
		lines = append(lines, fmt.Sprintf("credentials-file: %s", config.CredentialsFile))
	}
	lines = append(lines, "", "ingress:")
	for _, entry := range filterFallbackIngress(config.Ingress) {
		if entry.Hostname != "" {
			lines = append(lines, fmt.Sprintf("  - hostname: %s", entry.Hostname))
			lines = append(lines, fmt.Sprintf("    service: %s", entry.Service))
		} else {
			lines = append(lines, fmt.Sprintf("  - service: %s", entry.Service))
		}
		lines = append(lines, "")
	}
	if !hasFallbackIngress(config.Ingress) {
		lines = append(lines, "  - service: http_status:404", "")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
}

func normalizeIngressMap(input map[string]string, index int) IngressEntry {
	return normalizeIngressEntry(IngressEntry{
		ID:       input["id"],
		Hostname: input["hostname"],
		Service:  input["service"],
	}, index)
}

func normalizeIngressEntry(entry IngressEntry, index int) IngressEntry {
	id := strings.TrimSpace(entry.ID)
	if id == "" {
		b := make([]byte, 6)
		_, _ = rand.Read(b)
		id = fmt.Sprintf("ingress-%d-%s", time.Now().UnixNano(), hex.EncodeToString(b))
	}
	return IngressEntry{
		ID:       id,
		Hostname: strings.TrimSpace(entry.Hostname),
		Service:  strings.TrimSpace(entry.Service),
	}
}

func validateIngressItem(item IngressEntry) error {
	if strings.TrimSpace(item.Service) == "" {
		return fmt.Errorf("service is required")
	}
	return nil
}

func filterFallbackIngress(entries []IngressEntry) []IngressEntry {
	result := make([]IngressEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Service == "http_status:404" && entry.Hostname == "" {
			continue
		}
		result = append(result, entry)
	}
	return result
}

func hasFallbackIngress(entries []IngressEntry) bool {
	for _, entry := range entries {
		if entry.Service == "http_status:404" && entry.Hostname == "" {
			return true
		}
	}
	return false
}

func buildRouteDNSArgs(tunnel *Tunnel, hostname string) []string {
	return []string{
		"tunnel",
		"--config",
		tunnel.ConfigPath,
		"--origincert",
		tunnel.CredentialPath,
		"route",
		"dns",
		tunnel.TunnelName,
		hostname,
	}
}

var _ = pluginpkg.Metadata{}
