package redis

// Mode specifies the Redis deployment mode.
type Mode string

const (
	ModeSingle   Mode = "single"
	ModeCluster  Mode = "cluster"
	ModeSentinel Mode = "sentinel"
)

// Config defines Redis-specific configuration.
type Config struct {
	// Connection settings
	Mode       Mode   `json:"mode" yaml:"mode"`
	Addr       string `json:"addr" yaml:"addr"`
	Password   string `json:"password" yaml:"password"`
	TLSEnabled bool   `json:"tls_enabled" yaml:"tlsEnabled"`

	// Sentinel-specific settings
	MasterName string `json:"master_name" yaml:"masterName"`
}
