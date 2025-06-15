package network

import (
	"context"
	"net"
	"time"
)

// Network defines the interface for mesh networking
type Network interface {
	// Node management
	Join(ctx context.Context, config NodeConfig) error
	Leave(ctx context.Context) error
	GetNodeInfo(ctx context.Context) (*NodeInfo, error)
	ListNodes(ctx context.Context) ([]*NodeInfo, error)

	// Connectivity
	Connect(ctx context.Context, nodeID string) error
	Disconnect(ctx context.Context, nodeID string) error
	IsConnected(ctx context.Context, nodeID string) (bool, error)
	GetConnections(ctx context.Context) ([]*Connection, error)

	// Network configuration
	SetACL(ctx context.Context, rules []ACLRule) error
	GetACL(ctx context.Context) ([]ACLRule, error)
	UpdateRoutes(ctx context.Context, routes []Route) error

	// Health and monitoring
	Ping(ctx context.Context, nodeID string) (*PingResult, error)
	GetNetworkStats(ctx context.Context) (*NetworkStats, error)
	HealthCheck(ctx context.Context) error
	Close() error
}

// NodeConfig defines configuration for joining the network
type NodeConfig struct {
	NodeID     string            `json:"nodeId"`
	Name       string            `json:"name,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	Namespace  string            `json:"namespace,omitempty"`
	AuthKey    string            `json:"authKey,omitempty"`
	Hostname   string            `json:"hostname,omitempty"`
	Advertise  []string          `json:"advertise,omitempty"` // IP addresses to advertise
	Routes     []Route           `json:"routes,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
	Ephemeral  bool              `json:"ephemeral,omitempty"` // Node disappears when offline
}

// NodeInfo represents information about a network node
type NodeInfo struct {
	ID           string            `json:"id"`
	Name         string            `json:"name,omitempty"`
	IP           net.IP            `json:"ip"`
	Hostname     string            `json:"hostname,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Namespace    string            `json:"namespace,omitempty"`
	Online       bool              `json:"online"`
	LastSeen     time.Time         `json:"lastSeen"`
	JoinedAt     time.Time         `json:"joinedAt"`
	Version      string            `json:"version,omitempty"`
	OS           string            `json:"os,omitempty"`
	Arch         string            `json:"arch,omitempty"`
	Location     *Location         `json:"location,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Properties   map[string]string `json:"properties,omitempty"`
	Ephemeral    bool              `json:"ephemeral"`
}

// Location represents geographic location information
type Location struct {
	Country   string  `json:"country,omitempty"`
	Region    string  `json:"region,omitempty"`
	City      string  `json:"city,omitempty"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
}

// Connection represents a network connection between nodes
type Connection struct {
	NodeID        string            `json:"nodeId"`
	RemoteIP      net.IP            `json:"remoteIp"`
	LocalIP       net.IP            `json:"localIp"`
	Protocol      string            `json:"protocol"` // "tcp", "udp", "derp"
	Established   time.Time         `json:"established"`
	LastActivity  time.Time         `json:"lastActivity"`
	BytesSent     int64             `json:"bytesSent"`
	BytesReceived int64             `json:"bytesReceived"`
	Latency       time.Duration     `json:"latency"`
	Quality       ConnectionQuality `json:"quality"`
}

// ConnectionQuality defines the quality of a connection
type ConnectionQuality string

const (
	ConnectionQualityExcellent ConnectionQuality = "excellent" // < 50ms, no loss
	ConnectionQualityGood      ConnectionQuality = "good"      // < 100ms, < 1% loss
	ConnectionQualityFair      ConnectionQuality = "fair"      // < 200ms, < 5% loss
	ConnectionQualityPoor      ConnectionQuality = "poor"      // > 200ms or > 5% loss
	ConnectionQualityUnknown   ConnectionQuality = "unknown"
)

// ACLRule defines an access control rule
type ACLRule struct {
	ID          string      `json:"id,omitempty"`
	Description string      `json:"description,omitempty"`
	Action      ACLAction   `json:"action"`
	Source      ACLSelector `json:"source"`
	Destination ACLSelector `json:"destination"`
	Ports       []string    `json:"ports,omitempty"`     // "80", "443", "8000-9000"
	Protocols   []string    `json:"protocols,omitempty"` // "tcp", "udp", "icmp"
	Priority    int         `json:"priority,omitempty"`  // Higher = more important
}

// ACLAction defines the action to take for a rule
type ACLAction string

const (
	ACLActionAllow ACLAction = "allow"
	ACLActionDeny  ACLAction = "deny"
)

// ACLSelector defines how to select nodes for ACL rules
type ACLSelector struct {
	NodeIDs    []string          `json:"nodeIds,omitempty"`
	Tags       []string          `json:"tags,omitempty"`
	Namespaces []string          `json:"namespaces,omitempty"`
	IPRanges   []string          `json:"ipRanges,omitempty"` // CIDR notation
	Properties map[string]string `json:"properties,omitempty"`
	All        bool              `json:"all,omitempty"` // Select all nodes
}

// Route defines a network route
type Route struct {
	Destination string `json:"destination"` // CIDR notation
	Gateway     string `json:"gateway,omitempty"`
	Interface   string `json:"interface,omitempty"`
	Metric      int    `json:"metric,omitempty"`
	Advertise   bool   `json:"advertise,omitempty"` // Advertise to other nodes
}

// PingResult represents the result of a ping operation
type PingResult struct {
	NodeID     string        `json:"nodeId"`
	Success    bool          `json:"success"`
	Latency    time.Duration `json:"latency"`
	PacketLoss float64       `json:"packetLoss"` // 0.0 to 1.0
	Error      string        `json:"error,omitempty"`
	Timestamp  time.Time     `json:"timestamp"`
}

// NetworkStats represents network statistics
type NetworkStats struct {
	NodeCount          int                       `json:"nodeCount"`
	OnlineNodes        int                       `json:"onlineNodes"`
	Connections        int                       `json:"connections"`
	TotalBytesSent     int64                     `json:"totalBytesSent"`
	TotalBytesReceived int64                     `json:"totalBytesReceived"`
	AverageLatency     time.Duration             `json:"averageLatency"`
	PacketLoss         float64                   `json:"packetLoss"`
	Uptime             time.Duration             `json:"uptime"`
	ByNamespace        map[string]NamespaceStats `json:"byNamespace,omitempty"`
	TopConnections     []*Connection             `json:"topConnections,omitempty"`
}

// NamespaceStats represents statistics for a specific namespace
type NamespaceStats struct {
	NodeCount      int           `json:"nodeCount"`
	OnlineNodes    int           `json:"onlineNodes"`
	BytesSent      int64         `json:"bytesSent"`
	BytesReceived  int64         `json:"bytesReceived"`
	AverageLatency time.Duration `json:"averageLatency"`
}

// NetworkConfig defines configuration for the network system
type NetworkConfig struct {
	Provider     string            `json:"provider"` // "tailscale", "wireguard", "libp2p"
	AuthKey      string            `json:"authKey,omitempty"`
	ControlURL   string            `json:"controlUrl,omitempty"`
	Hostname     string            `json:"hostname,omitempty"`
	Advertise    []string          `json:"advertise,omitempty"`
	AcceptRoutes bool              `json:"acceptRoutes,omitempty"`
	AcceptDNS    bool              `json:"acceptDns,omitempty"`
	ExitNode     bool              `json:"exitNode,omitempty"`
	ShieldsUp    bool              `json:"shieldsUp,omitempty"` // Block incoming connections
	Properties   map[string]string `json:"properties,omitempty"`
}

// NetworkEvent represents a network event
type NetworkEvent struct {
	Type      NetworkEventType       `json:"type"`
	NodeID    string                 `json:"nodeId"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// NetworkEventType defines types of network events
type NetworkEventType string

const (
	NetworkEventNodeJoined     NetworkEventType = "node_joined"
	NetworkEventNodeLeft       NetworkEventType = "node_left"
	NetworkEventNodeOnline     NetworkEventType = "node_online"
	NetworkEventNodeOffline    NetworkEventType = "node_offline"
	NetworkEventConnectionUp   NetworkEventType = "connection_up"
	NetworkEventConnectionDown NetworkEventType = "connection_down"
	NetworkEventRouteAdded     NetworkEventType = "route_added"
	NetworkEventRouteRemoved   NetworkEventType = "route_removed"
	NetworkEventACLUpdated     NetworkEventType = "acl_updated"
)

// TrafficStats represents traffic statistics for a node or connection
type TrafficStats struct {
	BytesSent       int64         `json:"bytesSent"`
	BytesReceived   int64         `json:"bytesReceived"`
	PacketsSent     int64         `json:"packetsSent"`
	PacketsReceived int64         `json:"packetsReceived"`
	Errors          int64         `json:"errors"`
	Drops           int64         `json:"drops"`
	Duration        time.Duration `json:"duration"`
	StartTime       time.Time     `json:"startTime"`
	EndTime         time.Time     `json:"endTime"`
}

// Subnet represents a network subnet
type Subnet struct {
	CIDR        string    `json:"cidr"`
	Gateway     string    `json:"gateway,omitempty"`
	Namespace   string    `json:"namespace,omitempty"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	Isolated    bool      `json:"isolated"` // Isolated from other subnets
}
