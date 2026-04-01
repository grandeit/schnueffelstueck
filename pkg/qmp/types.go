package qmp

import "encoding/json"

// command is the client-to-server message format.
// Source: docs/interop/qmp-spec.rst "Issuing Commands"
type command struct {
	Execute   string          `json:"execute"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
	ID        json.RawMessage `json:"id,omitempty"`
}

// response covers success replies, error replies, and async events.
// Source: docs/interop/qmp-spec.rst "Commands Responses" and "Asynchronous events"
type response struct {
	Return    json.RawMessage `json:"return,omitempty"`
	Error     *qmpError       `json:"error,omitempty"`
	ID        json.RawMessage `json:"id,omitempty"`
	Event     string          `json:"event,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Timestamp *timestamp      `json:"timestamp,omitempty"`
}

// qmpError is the nested error object inside an error response.
// Source: docs/interop/qmp-spec.rst "Error"
type qmpError struct {
	Class string `json:"class"`
	Desc  string `json:"desc"`
}

// timestamp is the time attached to async events.
// Source: docs/interop/qmp-spec.rst "Asynchronous events"
type timestamp struct {
	Seconds      int64 `json:"seconds"`
	Microseconds int64 `json:"microseconds"`
}

// greeting is the server's initial message on connection.
// Source: qapi/control.json (VersionInfo, VersionTriple)
type greeting struct {
	QMP *struct {
		Version struct {
			QEMU struct {
				Major int `json:"major"`
				Minor int `json:"minor"`
				Micro int `json:"micro"`
			} `json:"qemu"`
			Package string `json:"package"`
		} `json:"version"`
		Capabilities []string `json:"capabilities"`
	} `json:"QMP,omitempty"`
}

// BalloonInfo is the return type of query-balloon.
// Source: qapi/machine.json (BalloonInfo struct)
type BalloonInfo struct {
	Actual uint64 `json:"actual"`
}

// GuestStats is the return type of qom-get guest-stats.
// Source: hw/virtio/virtio-balloon.c (balloon_stats_get_all)
type GuestStats struct {
	Stats      GuestStatsData `json:"stats"`
	LastUpdate int64          `json:"last-update"`
}

// GuestStatsData contains all 16 virtio-balloon stat fields.
// Unsupported stats are reported as UINT64_MAX by QEMU.
// Source: hw/virtio/virtio-balloon.c (balloon_stat_names[])
type GuestStatsData struct {
	SwapIn          uint64 `json:"stat-swap-in"`
	SwapOut         uint64 `json:"stat-swap-out"`
	MajorFaults     uint64 `json:"stat-major-faults"`
	MinorFaults     uint64 `json:"stat-minor-faults"`
	FreeMemory      uint64 `json:"stat-free-memory"`
	TotalMemory     uint64 `json:"stat-total-memory"`
	AvailableMemory uint64 `json:"stat-available-memory"`
	DiskCaches      uint64 `json:"stat-disk-caches"`
	HtlbPgalloc     uint64 `json:"stat-htlb-pgalloc"`
	HtlbPgfail      uint64 `json:"stat-htlb-pgfail"`
	OOMKills        uint64 `json:"stat-oom-kills"`
	AllocStalls     uint64 `json:"stat-alloc-stalls"`
	AsyncScans      uint64 `json:"stat-async-scans"`
	DirectScans     uint64 `json:"stat-direct-scans"`
	AsyncReclaims   uint64 `json:"stat-async-reclaims"`
	DirectReclaims  uint64 `json:"stat-direct-reclaims"`
}
