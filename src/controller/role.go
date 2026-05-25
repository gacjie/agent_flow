package controller

import (
	"log/slog"
	"net/http"
	"strconv"

	"agent_flow/src/middleware"
	"agent_flow/src/service"

	"github.com/go-chi/chi/v5"
)

// RoleCtrl 角色管理控制器
type RoleCtrl struct {
	Base
	PermService *service.PermissionService
}

// List 角色列表
func (c *RoleCtrl) List(w http.ResponseWriter, r *http.Request) {
	roles, err := c.PermService.ListRoles()
	if err != nil {
		c.RenderError(w, http.StatusInternalServerError, "获取角色列表失败")
		return
	}

	data := map[string]interface{}{
		"Title":      "角色管理",
		"Roles":      roles,
		"AdminPage":  true,
		"ActiveMenu": "roles",
	}
	c.Render(w, r, "admin/role_list", data)
}

// EditForm 编辑角色权限
func (c *RoleCtrl) EditForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的角色 ID")
		return
	}

	role, err := c.PermService.GetRole(uint(id))
	if err != nil {
		c.RenderError(w, http.StatusNotFound, "角色不存在")
		return
	}

	allPerms, _ := c.PermService.ListPermissions()
	roleCodes, _ := c.PermService.GetRolePermissionCodes(uint(id))

	// 转为 map 方便模板判断
	codeMap := make(map[string]bool)
	for _, code := range roleCodes {
		codeMap[code] = true
	}

	data := map[string]interface{}{
		"Title":       "编辑角色权限 - " + role.Label,
		"FormRole":    role,
		"AllPerms":    allPerms,
		"RolePerms":   codeMap,
		"AdminPage":   true,
		"ActiveMenu":  "roles",
	}
	c.Render(w, r, "admin/role_edit", data)
}

// Update 保存角色权限
func (c *RoleCtrl) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseUint(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		c.RenderError(w, http.StatusBadRequest, "无效的角色 ID")
		return
	}

	if err := r.ParseForm(); err != nil {
		c.RenderError(w, http.StatusBadRequest, "表单解析失败")
		return
	}

	codes := r.Form["permissions"]

	if err := c.PermService.SetRolePermissions(uint(id), codes); err != nil {
		c.RenderError(w, http.StatusInternalServerError, "保存权限失败")
		return
	}

	operator := middleware.GetCurrentAdmin(r)
	slog.Info("管理员操作:修改角色权限", "operator", operator.Username, "role_id", id, "permissions", codes)

	// 刷新缓存
	c.PermService.Reload()

	c.Redirect(w, r, "/admin/roles")
}
