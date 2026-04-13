package tunnel

import "time"

// Tunnel represents a managed Cloudflare tunnel record.
type Tunnel struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	Name                string    `gorm:"size:255;not null" json:"name"`
	TunnelName          string    `gorm:"size:255;not null" json:"tunnel_name"`
	ConfigPath          string    `gorm:"size:1024;not null;uniqueIndex" json:"config_path"`
	CredentialPath      string    `gorm:"size:1024;not null" json:"credential_path"`
	SharedCredentialKey string    `gorm:"size:255" json:"shared_credential_key"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (Tunnel) TableName() string {
	return "plugin_tunnel_tunnels"
}

// IngressEntry represents one ingress row parsed from a cloudflared config.
type IngressEntry struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	Service  string `json:"service"`
}

// TunnelConfig is the parsed cloudflared config exposed to the frontend.
type TunnelConfig struct {
	Path            string         `json:"path"`
	TunnelName      string         `json:"tunnel_name"`
	CredentialsFile string         `json:"credentials_file"`
	Ingress         []IngressEntry `json:"ingress"`
}
