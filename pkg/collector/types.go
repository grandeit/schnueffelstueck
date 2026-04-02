package collector

import "time"

type GuestMemory struct {
	Balloon        uint64    `json:"balloon"`
	Total          uint64    `json:"total"`
	Free           uint64    `json:"free"`
	Available      uint64    `json:"available"`
	SwapIn         uint64    `json:"swap_in"`
	SwapOut        uint64    `json:"swap_out"`
	MajorFaults    uint64    `json:"major_faults"`
	MinorFaults    uint64    `json:"minor_faults"`
	DiskCaches     uint64    `json:"disk_caches"`
	HtlbPgalloc    uint64    `json:"htlb_pgalloc"`
	HtlbPgfail     uint64    `json:"htlb_pgfail"`
	OOMKills       uint64    `json:"oom_kills"`
	AllocStalls    uint64    `json:"alloc_stalls"`
	AsyncScans     uint64    `json:"async_scans"`
	DirectScans    uint64    `json:"direct_scans"`
	AsyncReclaims  uint64    `json:"async_reclaims"`
	DirectReclaims uint64    `json:"direct_reclaims"`
	LastUpdate     time.Time `json:"last_update"`
}

type HostMemory struct {
	Total     uint64 `json:"total"`
	Free      uint64 `json:"free"`
	Available uint64 `json:"available"`
	SwapTotal uint64 `json:"swap_total"`
	SwapFree  uint64 `json:"swap_free"`
}

type Sample struct {
	Timestamp time.Time   `json:"timestamp"`
	Guest     GuestMemory `json:"guest"`
	Host      HostMemory  `json:"host"`
}
