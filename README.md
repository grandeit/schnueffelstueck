# Schnüffelstück

_Ein selbsttätiger Entlüfter, in Fachkreisen auch Schnüffelstück genannt._

---

A KubeVirt sidecar that dynamically manages VM memory using QEMU balloon devices.
It monitors host and guest memory and automatically inflates or deflates
the balloon to balance memory across VMs on the same node. Two controllers
are available: a **pressure** controller that uses exponential curves and guest
awareness for fine-grained control, and a simpler **watermark** controller that
uses threshold-based hysteresis.

## How It Works

Schnüffelstück runs as a [hook sidecar](https://kubevirt.io/user-guide/user_workloads/hook-sidecar/) inside the `virt-launcher` pod. On VM startup it:

1. Injects a QMP (QEMU Monitor Protocol) chardev into the domain XML via KubeVirt's `OnDefineDomain` hook
2. Connects to the QEMU monitor socket
3. Runs a control loop that samples host and guest memory, computes a balloon target, and applies it via QMP

```
┌────────────────────── virt-launcher pod ───────────────────────┐
│                                                                │
│  ┌──────────────┐    QMP socket   ┌─────────────────────────┐  │
│  │ schnüffel-   │◄───────────────►│        QEMU / VM        │  │
│  │ stück        │                 │  ┌───────────────────┐  │  │
│  │              │                 │  │  virtio-balloon   │  │  │
│  │  collect ─┐  │                 │  │  driver           │  │  │
│  │  decide   │  │                 │  └───────────────────┘  │  │
│  │  apply  ◄─┘  │                 └─────────────────────────┘  │
│  └──────┬───────┘                                              │
│         │ /proc/meminfo                                        │
│         ▼                                                      │
│     host memory                                                │
└────────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- KubeVirt with the **Sidecar** feature gate enabled:
  ```yaml
  apiVersion: kubevirt.io/v1
  kind: KubeVirt
  spec:
    configuration:
      developerConfiguration:
        featureGates:
          - Sidecar
  ```
- The guest OS must have a **virtio-balloon driver** (Linux has it built-in, Windows needs [virtio-win](https://fedorapeople.org/groups/virt/virtio-win/direct-downloads/))

### Deploy

Add the hook sidecar annotation to your VMI and configure guest memory:

```yaml
apiVersion: kubevirt.io/v1
kind: VirtualMachineInstance
metadata:
  name: my-vm
  annotations:
    hooks.kubevirt.io/hookSidecars: |
      [{
        "image": "ghcr.io/grandeit/schnueffelstueck:latest",
        "imagePullPolicy": "Always",
        "command": ["/schnueffelstueck", "--log-level", "debug"]
      }]
    schnueffelstueck/controller: "pressure"
    schnueffelstueck/guest-overcommit: "2.0"
spec:
  domain:
    resources:
      requests:
        memory: 6Gi
    memory:
      guest: 12Gi
```

- `memory.guest` is the VM's full RAM size (what QEMU allocates)
- `resources.requests.memory` is the Kubernetes pod resource request, used for scheduling and cgroup limits - set it to `guest / overcommit` so the pod only reserves the minimum it needs
- The balloon floor (minimum VM size) is enforced by schnüffelstück's `guest-overcommit` setting
- The `command` field is optional - omit it to use the default log level (`info`)

> **Note:** KubeVirt automatically adds ~300 MiB of memory overhead to the pod's resource requests for QEMU, libvirt, and virt-launcher processes. You don't need to account for this - just set `requests.memory` to the balloon floor you want.

> **Note:** When using a `VirtualMachine` resource instead of a bare `VirtualMachineInstance`, place the annotations in the VMI template:
> ```yaml
> apiVersion: kubevirt.io/v1
> kind: VirtualMachine
> metadata:
>   name: my-vm
> spec:
>   template:
>     metadata:
>       annotations:
>         hooks.kubevirt.io/hookSidecars: |
>           [{"image": "ghcr.io/grandeit/schnueffelstueck:latest"}]
>         schnueffelstueck/controller: "pressure"
>     spec:
>       domain:
>         # ...
> ```

## Controllers

### `log` (default)

Logs host and guest memory statistics without taking any action. Useful for observing memory behavior before enabling ballooning.

### `pressure`

Adjusts the balloon based on two signals:

- **Pressure** (0–1): how urgently the host needs memory, derived from host free % and an exponential curve
- **Generosity** (0–1): how willing the guest is to give up memory. A guest is fully generous (1.0) when all its used memory fits inside its reserved floor (`1/overcommit` of total RAM) - meaning everything above the floor is surplus. As the guest uses more memory and exceeds its floor, generosity drops toward 0

As host pressure rises, a lerp override increasingly forces generosity toward 1.0, guaranteeing the host can always reclaim up to the overcommit limit:

```
reclaim = pressure × (generosity + pressure × (1 − generosity)) × maxReclaim × guestTotal
```

Where `maxReclaim = 1 − 1/overcommit` (e.g., 0.5 for overcommit 2).

![Pressure curves](doc/pressure.png)

**Top row**: base exponential function, host pressure vs host free %, and guest generosity vs guest free %.
**Middle row**: effective reclaim % at pressure 0.2, 0.5, and 0.9 with steepness s=2. Despite lower generosity, higher overcommit reclaims more because the reclaimable zone is larger. At high pressure the lerp override forces reclaim toward the maximum.
**Bottom row**: same as middle row but with steepness s=8 — reclaim stays suppressed until the guest has significantly more free memory, then ramps sharply.

#### Capacity Planning

At maximum pressure, each VM gives up `maxReclaim` of its RAM. The host reserve is guaranteed:

```
max VMs = (host RAM × (1 − hostReservedPct)) / (VM size × (1 / overcommit))
```

Example: 64 GiB host, 10% reserved, 12 GiB VMs, overcommit 2:

```
(64 × 0.9) / (12 × 0.5) = 57.6 / 6 = 9 VMs
```

This is a worst-case guarantee. In normal operation (low pressure), VMs keep more memory.

### `watermark`

A simpler alternative to the pressure controller. Instead of continuous exponential curves, it uses two thresholds (watermarks) with a dead band in between:

- **Below low watermark**: host is running low on memory — reclaim from VMs proportionally
- **Above high watermark**: host has excess memory — release it back to VMs
- **Between low and high**: do nothing (dead band prevents oscillation)

```
reclaim = clamp((targetFree − hostFree) / vmFraction, 0, maxReclaim) × guestTotal
```

Where `vmFraction = guestTotal / hostTotal` scales the host-level deficit to this VM's fair share, and `maxReclaim = 1 − 1/overcommit` enforces the balloon floor.

The watermark controller doesn't consider guest memory state — it only looks at host free %. Guest protection comes solely from the overcommit floor. This makes it predictable and easy to reason about, but less adaptive than the pressure controller.

![Watermark curves](doc/watermark.png)

Each panel shows different VM sizes (4–32G) on a 64G host with oc=2, low=10%, high=20%. **Left**: reclaim as a percentage of guest RAM — small VMs hit the 50% ceiling quickly, large VMs never reach it because the formula asks less of them proportionally. **Right**: absolute GiB reclaimed — larger VMs still contribute more in absolute terms despite the lower percentage.

Setting `watermark-high-pct = watermark-low-pct` collapses the dead band, turning this into a simple target-chaser.

#### Capacity Planning

At maximum reclaim, each VM keeps `guestSize / overcommit` of host RAM. The low watermark is maintained as long as:

```
max VMs = (host RAM × (1 − lowWatermark)) / (VM size / overcommit)
```

Example: 64 GiB host, low=10%, 8 GiB VMs, overcommit 2:

```
(64 × 0.90) / (8 / 2) = 57.6 / 4 = 14 VMs
```

This is the same formula as the pressure controller's capacity planning, with `lowWatermark` replacing `hostReservedPct`.

## Configuration

All settings are VMI annotations with the `schnueffelstueck/` prefix.

### General

| Annotation | Type | Default | Description |
|---|---|---|---|
| `controller` | string | `log` | Controller kind: `log`, `pressure`, or `watermark` |
| `interval` | duration | `1s` | Control loop tick interval |
| `dry-run` | bool | `false` | Log decisions without applying them |
| `guest-overcommit` | float | `2.0` | Overcommit ratio. `2.0` = VM keeps at least 50% of its RAM |
| `guest-max-step-pct` | float | `0.1` | Max balloon change per tick (fraction of guest total) |
| `guest-min-step-pct` | float | `0.01` | Dead band - changes smaller than this are skipped |
| `host-reserved-pct` | float | `0.1` | Host free % threshold below which pressure = 1.0 |
| `qemu-stats-period` | int | `1` | Guest balloon stats polling interval in seconds |

### Pressure Controller

| Annotation | Type | Default | Description |
|---|---|---|---|
| `pressure-host-steepness` | float | `2` | Host pressure curve steepness. Higher = lazy until critical |
| `pressure-guest-steepness` | float | `2` | Guest generosity curve steepness. Higher = stingy with surplus |

Setting steepness to `0` gives a linear curve. Negative values make the curve concave (aggressive early, gentle late).

### Watermark Controller

| Annotation | Type | Default | Description |
|---|---|---|---|
| `watermark-high-pct` | float | `0.2` | Host free % above which memory is released back to VMs |
| `watermark-low-pct` | float | `0.1` | Host free % below which memory is reclaimed from VMs |

### CLI Flags

| Flag | Default | Description |
|---|---|---|
| `--log-level` | `info` | Log level: `debug`, `info`, `warn`, `error` |