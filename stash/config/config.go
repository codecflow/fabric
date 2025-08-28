package config

import (
	_ "embed"
	"os"

	"github.com/knadh/koanf/v2"
)

//go:embed default.yaml
var defaultConfig []byte

func DefaultConfig() []byte {
	return defaultConfig
}

type LoaderFunc func(k *koanf.Koanf) error

type BlobCacheConfig struct {
	Server  Server  `key:"server" json:"server"`
	Client  Client  `key:"client" json:"client"`
	Global  Global  `key:"global" json:"global"`
	Metrics Metrics `key:"metrics" json:"metrics"`
}

type Client struct {
	Token                 string `key:"token" json:"token"`
	MinRetryLengthBytes   int64  `key:"minRetryLengthBytes" json:"min_retry_length_bytes"`
	MaxGetContentAttempts int    `key:"maxGetContentAttempts" json:"max_get_content_attempts"`
	NTopHosts             int    `key:"nTopHosts" json:"n_top_hosts"`
	BlobFs                BlobFs `key:"blobfs" json:"blobfs"`
}

type BlobFs struct {
	Enabled            bool     `key:"enabled" json:"enabled"`
	MountPoint         string   `key:"mountPoint" json:"mount_point"`
	MaxBackgroundTasks int      `key:"maxBackgroundTasks" json:"max_background_tasks"`
	MaxWriteKB         int      `key:"maxWriteKB" json:"max_write_kb"`
	MaxReadAheadKB     int      `key:"maxReadAheadKB" json:"max_read_ahead_kb"`
	DirectMount        bool     `key:"directMount" json:"direct_mount"`
	DirectIO           bool     `key:"directIO" json:"direct_io"`
	Options            []string `key:"options" json:"options"`
}

type Global struct {
	DefaultLocality                 string  `key:"defaultLocality" json:"default_locality"`
	CoordinatorHost                 string  `key:"coordinatorHost" json:"coordinator_host"`
	ServerPort                      uint    `key:"serverPort" json:"server_port"`
	DiscoveryIntervalS              int     `key:"discoveryIntervalS" json:"discovery_interval_s"`
	RoundTripThresholdMilliseconds  uint    `key:"rttThresholdMilliseconds" json:"rtt_threshold_ms"`
	HostStorageCapacityThresholdPct float64 `key:"hostStorageCapacityThresholdPct" json:"host_storage_capacity_threshold_pct"`
	GRPCDialTimeoutS                int     `key:"grpcDialTimeoutS" json:"grpc_dial_timeout_s"`
	GRPCMessageSizeBytes            int     `key:"grpcMessageSizeBytes" json:"grpc_message_size_bytes"`
	GRPCInitialWindowSize           int     `key:"grpcInitialWindowSize" json:"grpc_initial_window_size"`
	GRPCInitialConnWindowSize       int     `key:"grpcInitialConnWindowSize" json:"grpc_initial_conn_window_size"`
	GRPCWriteBufferSize             int     `key:"grpcWriteBufferSize" json:"grpc_write_buffer_size"`
	GRPCReadBufferSize              int     `key:"grpcReadBufferSize" json:"grpc_read_buffer_size"`
	GRPCMaxConcurrentStreams        int     `key:"grpcMaxConcurrentStreams" json:"grpc_max_concurrent_streams"`
	GRPCNumStreamWorkers            int     `key:"grpcNumStreamWorkers" json:"grpc_num_stream_workers"`
	DebugMode                       bool    `key:"debugMode" json:"debug_mode"`
	PrettyLogs                      bool    `key:"prettyLogs" json:"pretty_logs"`
}

// GetLocality returns the default locality.
func (g *Global) GetLocality() string {
	locality := os.Getenv("STASH_LOCALITY")
	if locality != "" {
		return locality
	}

	return g.DefaultLocality
}

type Metrics struct {
	PushIntervalS int    `key:"pushIntervalS" json:"push_interval_s"`
	URL           string `key:"url" json:"url"`
	Username      string `key:"username" json:"username"`
	Password      string `key:"password" json:"password"`
}

type ServerMode string

const (
	ServerModeCoordinator ServerMode = "coordinator"
	ServerModeSlave       ServerMode = "slave"
)

type Source struct {
	Mode           string     `key:"mode" json:"mode"`
	FilesystemName string     `key:"fsName" json:"filesystem_name"`
	FilesystemPath string     `key:"fsPath" json:"filesystem_path"`
	JuiceFS        JuiceFS    `key:"juicefs" json:"juicefs"`
	MountPoint     MountPoint `key:"mountpoint" json:"mountpoint"`
}

type JuiceFS struct {
	RedisURI   string `key:"redisURI" json:"redis_uri"`
	Bucket     string `key:"bucket" json:"bucket"`
	AccessKey  string `key:"accessKey" json:"access_key"`
	SecretKey  string `key:"secretKey" json:"secret_key"`
	CacheSize  int64  `key:"cacheSize" json:"cache_size"`
	BlockSize  int64  `key:"blockSize" json:"block_size"`
	Prefetch   int64  `key:"prefetch" json:"prefetch"`
	BufferSize int64  `key:"bufferSize" json:"buffer_size"`
}

type MountPoint struct {
	BucketName     string `key:"bucketName" json:"bucket_name"`
	AccessKey      string `key:"accessKey" json:"access_key"`
	SecretKey      string `key:"secretKey" json:"secret_key"`
	Region         string `key:"region" json:"region"`
	EndpointURL    string `key:"endpointUrl" json:"endpoint_url"`
	ForcePathStyle bool   `key:"forcePathStyle" json:"force_path_style"`
}

type Server struct {
	Mode                  ServerMode `key:"mode" json:"mode"`
	DiskCacheDir          string     `key:"diskCacheDir" json:"disk_cache_dir"`
	DiskCacheMaxUsagePct  float64    `key:"diskCacheMaxUsagePct" json:"disk_cache_max_usage_pct"`
	EnableMemoryCache     bool       `key:"enableMemoryCache" json:"enable_memory_cache"`
	Token                 string     `key:"token" json:"token"`
	PrettyLogs            bool       `key:"prettyLogs" json:"pretty_logs"`
	ObjectTtlS            int        `key:"objectTtlS" json:"object_ttl_s"`
	MaxCachePct           int64      `key:"maxCachePct" json:"max_cache_pct"`
	PageSizeBytes         int64      `key:"pageSizeBytes" json:"page_size_bytes"`
	Metadata              Metadata   `key:"metadata" json:"metadata"`
	Sources               []Source   `key:"sources" json:"sources"`
	S3DownloadConcurrency int64      `key:"s3DownloadConcurrency" json:"s3_download_concurrency"`
	S3DownloadChunkSize   int64      `key:"s3DownloadChunkSize" json:"s3_download_chunk_size"`

	// Allows a coordinator to override a slave server's config for a specific locality/region
	Regions map[string]Region `key:"regions" json:"regions"`
}

type ValkeyExistingPrimary struct {
	Host string `key:"host" json:"host"`
	Port int    `key:"port" json:"port"`
}

type ValkeyConfig struct {
	PrimaryName     string                `key:"primaryName" json:"primary_name"`
	Password        string                `key:"password" json:"password"`
	TLS             bool                  `key:"tls" json:"tls"`
	Host            string                `key:"host" json:"host"`
	Port            int                   `key:"port" json:"port"`
	ExistingPrimary ValkeyExistingPrimary `key:"existingPrimary" json:"existingPrimary"`
}

type MetadataMode string

const (
	MetadataModeDefault MetadataMode = "default"
	MetadataModeLocal   MetadataMode = "local"
)

type RedisMode string

var (
	RedisModeSingle   RedisMode = "single"
	RedisModeCluster  RedisMode = "cluster"
	RedisModeSentinel RedisMode = "sentinel"
)

type Metadata struct {
	Mode         MetadataMode `key:"mode" json:"mode"`
	ValkeyConfig ValkeyConfig `key:"valkey" json:"valkey"`

	// Default config
	RedisAddr       string    `key:"redisAddr" json:"redis_addr"`
	RedisPasswd     string    `key:"redisPasswd" json:"redis_passwd"`
	RedisTLSEnabled bool      `key:"redisTLSEnabled" json:"redis_tls_enabled"`
	RedisMode       RedisMode `key:"redisMode" json:"redis_mode"`
	RedisMasterName string    `key:"redisMasterName" json:"redis_master_name"`
}

type Region struct {
	ServerConfig Server `key:"server" json:"server"`
}
