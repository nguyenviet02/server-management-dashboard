package tunnel

import (
	"fmt"

	pluginpkg "github.com/nguyenviet02/server-management-dashboard/internal/plugin"
)

type Plugin struct {
	svc     *Service
	handler *Handler
}

func New() *Plugin {
	return &Plugin{}
}

func (p *Plugin) Metadata() pluginpkg.Metadata {
	return pluginpkg.Metadata{
		ID:          "tunnel",
		Name:        "Tunnel",
		Version:     "1.0.0",
		Description: "Manage multiple Cloudflare tunnels and ingress routes",
		Author:      "ServerDash",
		Priority:    18,
		Icon:        "Waypoints",
		Category:    "deploy",
	}
}

func (p *Plugin) Init(ctx *pluginpkg.Context) error {
	if err := ctx.DB.AutoMigrate(&Tunnel{}); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	p.svc = NewService(ctx.DB, ctx.Logger)
	p.handler = NewHandler(p.svc)

	r := ctx.Router
	a := ctx.AdminRouter

	r.GET("/tunnels", p.handler.ListTunnels)
	r.GET("/tunnels/:id", p.handler.GetTunnel)
	r.GET("/tunnels/:id/config", p.handler.GetTunnelConfig)
	r.GET("/tunnels/:id/service-status", p.handler.ServiceStatus)

	a.POST("/tunnels", p.handler.CreateTunnel)
	a.PUT("/tunnels/:id", p.handler.UpdateTunnel)
	a.DELETE("/tunnels/:id", p.handler.DeleteTunnel)
	a.POST("/tunnels/:id/ingress", p.handler.AddIngress)
	a.PUT("/tunnels/:id/ingress", p.handler.UpdateIngress)
	a.DELETE("/tunnels/:id/ingress", p.handler.DeleteIngress)
	a.POST("/tunnels/:id/route-dns", p.handler.RouteDNS)
	a.POST("/tunnels/:id/service-restart", p.handler.RestartService)

	ctx.Logger.Info("Tunnel plugin routes registered")
	return nil
}

func (p *Plugin) Start() error {
	return nil
}

func (p *Plugin) Stop() error {
	return nil
}

func (p *Plugin) FrontendManifest() pluginpkg.FrontendManifest {
	return pluginpkg.FrontendManifest{
		ID: "tunnel",
		Routes: []pluginpkg.FrontendRoute{
			{Path: "/tunnels", Component: "TunnelList", Menu: true, Icon: "Waypoints", Label: "Tunnels"},
			{Path: "/tunnels/:id", Component: "TunnelDetail", Label: "Tunnel Detail"},
		},
		MenuGroup: "deploy",
		MenuOrder: 18,
	}
}

var (
	_ pluginpkg.Plugin           = (*Plugin)(nil)
	_ pluginpkg.FrontendProvider = (*Plugin)(nil)
)
