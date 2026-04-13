package tunnel

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) ListTunnels(c *gin.Context) {
	tunnels, err := h.svc.ListTunnels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tunnels": tunnels})
}

func (h *Handler) GetTunnel(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		return
	}
	tunnel, err := h.svc.GetTunnel(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tunnel not found"})
		return
	}
	c.JSON(http.StatusOK, tunnel)
}

func (h *Handler) CreateTunnel(c *gin.Context) {
	var req Tunnel
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tunnel, err := h.svc.CreateTunnel(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, tunnel)
}

func (h *Handler) UpdateTunnel(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		return
	}
	var req Tunnel
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tunnel, err := h.svc.UpdateTunnel(id, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tunnel)
}

func (h *Handler) DeleteTunnel(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		return
	}
	if err := h.svc.DeleteTunnel(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) GetTunnelConfig(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		return
	}
	tunnel, config, err := h.svc.GetTunnelConfig(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tunnel": tunnel, "config": config})
}

func (h *Handler) AddIngress(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		return
	}
	var req struct {
		Item     IngressEntry `json:"item" binding:"required"`
		RouteDNS *bool        `json:"route_dns"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	routeDNS := req.RouteDNS == nil || *req.RouteDNS
	tunnel, config, warning, err := h.svc.AddIngress(id, req.Item, routeDNS)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tunnel": tunnel, "config": config, "warning": warning})
}

func (h *Handler) UpdateIngress(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		return
	}
	var req struct {
		Item     IngressEntry `json:"item" binding:"required"`
		RouteDNS *bool        `json:"route_dns"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	routeDNS := req.RouteDNS == nil || *req.RouteDNS
	tunnel, config, warning, err := h.svc.UpdateIngress(id, req.Item, routeDNS)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tunnel": tunnel, "config": config, "warning": warning})
}

func (h *Handler) DeleteIngress(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		return
	}
	var req struct {
		ID string `json:"id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tunnel, config, warning, err := h.svc.DeleteIngress(id, req.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tunnel": tunnel, "config": config, "warning": warning})
}

func (h *Handler) RouteDNS(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		return
	}
	var req struct {
		Hostname string `json:"hostname" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.RouteDNS(id, req.Hostname); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *Handler) ServiceStatus(c *gin.Context) {
	c.JSON(http.StatusOK, h.svc.ServiceStatus())
}

func (h *Handler) RestartService(c *gin.Context) {
	if err := h.svc.RestartService(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func parseID(c *gin.Context) (uint, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0, err
	}
	return uint(id), nil
}
