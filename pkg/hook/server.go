package hook

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"google.golang.org/grpc"
	"libvirt.org/go/libvirtxml"

	"github.com/grandeit/schnueffelstueck/pkg/hook/info"
	"github.com/grandeit/schnueffelstueck/pkg/hook/v1alpha3"
)

const (
	hookSocketsDir = "/var/run/kubevirt-hooks"
	hookVersion    = "v1alpha3"
	hookName       = "schnueffelstueck"

	annotationPrefix   = "schnueffelstueck/"
	defaultStatsPeriod = 1

	qemuSocketName = "qemu.sock"
)

type Server struct {
	containerName         string
	socketPath            string
	ctx                   context.Context
	cancel                context.CancelFunc
	QEMUMonitorSocketPath string
	mu                    sync.Mutex
	settings              map[string]string
}

func NewServer() (*Server, error) {
	containerName := os.Getenv("CONTAINER_NAME")
	if containerName == "" {
		return nil, fmt.Errorf("CONTAINER_NAME environment variable is not set")
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{
		containerName:         containerName,
		socketPath:            filepath.Join(hookSocketsDir, hookName+".sock"),
		ctx:                   ctx,
		cancel:                cancel,
		QEMUMonitorSocketPath: filepath.Join(hookSocketsDir, qemuSocketName),
	}, nil
}

func (s *Server) Done() <-chan struct{} {
	return s.ctx.Done()
}

func (s *Server) SettingsFromAnnotations() map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

func (s *Server) Run() error {
	socket, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", s.socketPath, err)
	}
	defer os.Remove(s.socketPath)

	grpcServer := grpc.NewServer()
	info.RegisterInfoServer(grpcServer, &infoServer{})
	v1alpha3.RegisterCallbacksServer(grpcServer, &callbacksServer{
		server: s,
	})

	slog.Info("starting gRPC server for KubeVirt callbacks", "socket", s.socketPath, "version", hookVersion)

	errChan := make(chan error, 1)
	go func() {
		errChan <- grpcServer.Serve(socket)
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-sigChan:
		slog.Info("shutting down gRPC server, received signal", "signal", sig)
		s.cancel()
	case err := <-errChan:
		if err != nil {
			s.cancel()
			return fmt.Errorf("gRPC server error: %w", err)
		}
	case <-s.Done():
		slog.Info("shutting down gRPC server, received shutdown request")
	}

	grpcServer.GracefulStop()
	return nil
}

type infoServer struct{}

func (s *infoServer) Info(_ context.Context, _ *info.InfoParams) (*info.InfoResult, error) {
	slog.Info("received Info callback from KubeVirt")

	return &info.InfoResult{
		Name:     hookName,
		Versions: []string{hookVersion},
		HookPoints: []*info.HookPoint{
			{
				Name:     info.OnDefineDomainHookPointName,
				Priority: 0,
			},
			{
				Name:     info.ShutdownHookPointName,
				Priority: 0,
			},
		},
	}, nil
}

type callbacksServer struct {
	server *Server
}

func (s *callbacksServer) OnDefineDomain(_ context.Context, params *v1alpha3.OnDefineDomainParams) (*v1alpha3.OnDefineDomainResult, error) {
	slog.Info("received OnDefineDomain callback from KubeVirt")
	slog.Debug("received domain XML", "domain_xml", string(params.GetDomainXML()))

	settings, err := extractSettingsFromAnnotations(params.GetVmi())
	if err != nil {
		slog.Warn("failed to extract settings from VMI annotations, will use defaults", "error", err)
	} else {
		s.server.mu.Lock()
		s.server.settings = settings
		s.server.mu.Unlock()
		slog.Debug("extracted settings from VMI annotations", "settings", settings)
	}

	statsPeriod := defaultStatsPeriod
	if v, ok := settings["qemu-stats-period"]; ok {
		if p, err := strconv.Atoi(v); err == nil && p > 0 {
			statsPeriod = p
		} else {
			slog.Warn("ignoring invalid qemu-stats-period annotation", "value", v)
		}
	}

	domain := &libvirtxml.Domain{}
	if err := domain.Unmarshal(string(params.GetDomainXML())); err != nil {
		slog.Error("failed to parse domain XML", "error", err)
		return &v1alpha3.OnDefineDomainResult{DomainXML: params.GetDomainXML()}, nil
	}

	if domain.Devices == nil || domain.Devices.MemBalloon == nil {
		slog.Warn("domain XML has no memballoon device")
		return &v1alpha3.OnDefineDomainResult{DomainXML: params.GetDomainXML()}, nil
	}

	qemuSocketPath := filepath.Join(hookSocketsDir, s.server.containerName, qemuSocketName)

	if domain.QEMUCommandline == nil {
		domain.QEMUCommandline = &libvirtxml.DomainQEMUCommandline{}
	}

	if !slices.ContainsFunc(domain.QEMUCommandline.Args, func(arg libvirtxml.DomainQEMUCommandlineArg) bool {
		return strings.Contains(arg.Value, "socket,id="+hookName+",")
	}) {
		domain.QEMUCommandline.Args = append(domain.QEMUCommandline.Args,
			libvirtxml.DomainQEMUCommandlineArg{Value: "-chardev"},
			libvirtxml.DomainQEMUCommandlineArg{Value: "socket,id=" + hookName + ",path=" + qemuSocketPath + ",server=on,wait=off"},
			libvirtxml.DomainQEMUCommandlineArg{Value: "-mon"},
			libvirtxml.DomainQEMUCommandlineArg{Value: "chardev=" + hookName + ",mode=control"},
		)
	}

	domain.Devices.MemBalloon.Stats = &libvirtxml.DomainMemBalloonStats{
		Period: uint(statsPeriod),
	}

	domainXML, err := domain.Marshal()
	if err != nil {
		slog.Error("failed to marshal modified domain XML", "error", err)
		return &v1alpha3.OnDefineDomainResult{DomainXML: params.GetDomainXML()}, nil
	}

	slog.Info("returning modified domain XML to KubeVirt", "qemu_socket_path", qemuSocketPath, "qemu_stats_period", statsPeriod)
	slog.Debug("modified domain XML", "domain_xml", domainXML)

	return &v1alpha3.OnDefineDomainResult{
		DomainXML: []byte(domainXML),
	}, nil
}

func (s *callbacksServer) PreCloudInitIso(_ context.Context, params *v1alpha3.PreCloudInitIsoParams) (*v1alpha3.PreCloudInitIsoResult, error) {
	slog.Info("received PreCloudInitIso callback from KubeVirt")

	return &v1alpha3.PreCloudInitIsoResult{
		CloudInitData: params.GetCloudInitData(),
	}, nil
}

func (s *callbacksServer) Shutdown(_ context.Context, _ *v1alpha3.ShutdownParams) (*v1alpha3.ShutdownResult, error) {
	slog.Info("received Shutdown callback from KubeVirt")

	s.server.cancel()
	return &v1alpha3.ShutdownResult{}, nil
}

func extractSettingsFromAnnotations(vmiJSON []byte) (map[string]string, error) {
	var vmi struct {
		Metadata struct {
			Annotations map[string]string `json:"annotations"`
		} `json:"metadata"`
	}

	if err := json.Unmarshal(vmiJSON, &vmi); err != nil {
		return nil, fmt.Errorf("parsing VMI JSON: %w", err)
	}

	settings := make(map[string]string)

	for k, v := range vmi.Metadata.Annotations {
		if strings.HasPrefix(k, annotationPrefix) {
			settings[strings.TrimPrefix(k, annotationPrefix)] = v
		}
	}

	return settings, nil
}
