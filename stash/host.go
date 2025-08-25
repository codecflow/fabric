package stash

import (
	"time"

	"github.com/codecflow/fabric/stash/proto"
)

var HostPrefix string = "stash:host"

type Address struct {
	Public  string `redis:"public" json:"public"`
	Private string `redis:"private" json:"private"`
}

type Capacity struct {
	Used  int64 `redis:"used" json:"used"`
	Total int64 `redis:"total" json:"total"`
}

type Host struct {
	ID       string        `redis:"id" json:"id"`
	RTT      time.Duration `redis:"rtt" json:"rtt"`
	Address  Address       `redis:"address" json:"address"`
	Capacity Capacity      `redis:"capacity" json:"capacity"`
}

func (h *Host) Bytes() []byte {
	return []byte(h.ID)
}

func (h *Host) ToProto() *proto.Host {
	return &proto.Host{
		Id: h.ID,
		Address: &proto.Address{
			Public:  h.Address.Public,
			Private: h.Address.Private,
		},
		Capacity: &proto.Capacity{
			Used:  h.Capacity.Used,
			Total: h.Capacity.Total,
		},
	}
}
