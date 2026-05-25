package controller

import (
	"net/http"

	"agent_flow/src/common"
	"agent_flow/src/middleware"
	"agent_flow/src/model"
	"agent_flow/src/service"
)

// Profile 个人资料控制器
type Profile struct {
	Base
	AdminService *service.AdminService
}

// Edit 个人资料页面
func (c *Profile) Edit(w http.ResponseWriter, r *http.Request) {
	admin := middleware.GetCurrentAdmin(r)
	data := map[string]interface{}{
		"Title":      "个人资料",
		"FormAdmin":  admin,
		"AdminPage":  true,
		"ActiveMenu": "profile",
	}
	c.Render(w, r, "admin/profile", data)
}

// Update 更新个人资料
func (c *Profile) Update(w http.ResponseWriter, r *http.Request) {
	admin := middleware.GetCurrentAdmin(r)
	if err := r.ParseForm(); err != nil {
		c.RenderError(w, http.StatusBadRequest, "表单解析失败")
		return
	}

	req := &model.ProfileUpdateReq{
		Email:    r.FormValue("email"),
		Nickname: r.FormValue("nickname"),
	}

	if err := c.AdminService.UpdateProfile(admin.ID, req); err != nil {
		if appErr, ok := err.(*common.AppError); ok {
			data := map[string]interface{}{
				"Title":      "个人资料",
				"FormAdmin":  admin,
				"Error":      appErr.Message,
				"AdminPage":  true,
				"ActiveMenu": "profile",
			}
			c.Render(w, r, "admin/profile", data)
			return
		}
		c.RenderError(w, http.StatusInternalServerError, "更新失败")
		return
	}

	updated, _ := c.AdminService.GetByID(admin.ID)
	data := map[string]interface{}{
		"Title":      "个人资料",
		"FormAdmin":  updated,
		"Success":    "资料更新成功",
		"AdminPage":  true,
		"ActiveMenu": "profile",
	}
	c.Render(w, r, "admin/profile", data)
}

// PasswordForm 修改密码页面
func (c *Profile) PasswordForm(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":      "修改密码",
		"AdminPage":  true,
		"ActiveMenu": "password",
	}
	c.Render(w, r, "admin/password", data)
}

// PasswordUpdate 处理修改密码
func (c *Profile) PasswordUpdate(w http.ResponseWriter, r *http.Request) {
	admin := middleware.GetCurrentAdmin(r)
	if err := r.ParseForm(); err != nil {
		c.RenderError(w, http.StatusBadRequest, "表单解析失败")
		return
	}

	req := &model.ProfileUpdateReq{
		OldPassword: r.FormValue("old_password"),
		NewPassword: r.FormValue("new_password"),
	}

	if req.OldPassword == "" || req.NewPassword == "" {
		data := map[string]interface{}{
			"Title":      "修改密码",
			"Error":      "请填写原密码和新密码",
			"AdminPage":  true,
			"ActiveMenu": "password",
		}
		c.Render(w, r, "admin/password", data)
		return
	}

	// 密码复杂度校验
	if err := common.ValidatePassword(req.NewPassword); err != nil {
		appErr := err.(*common.AppError)
		data := map[string]interface{}{
			"Title":      "修改密码",
			"Error":      appErr.Message,
			"AdminPage":  true,
			"ActiveMenu": "password",
		}
		c.Render(w, r, "admin/password", data)
		return
	}

	if err := c.AdminService.UpdateProfile(admin.ID, req); err != nil {
		if appErr, ok := err.(*common.AppError); ok {
			data := map[string]interface{}{
				"Title":      "修改密码",
				"Error":      appErr.Message,
				"AdminPage":  true,
				"ActiveMenu": "password",
			}
			c.Render(w, r, "admin/password", data)
			return
		}
		c.RenderError(w, http.StatusInternalServerError, "修改失败")
		return
	}

	// 清除强制改密标记
	c.AdminService.ClearMustChangePassword(admin.ID)

	data := map[string]interface{}{
		"Title":      "修改密码",
		"Success":    "密码修改成功",
		"AdminPage":  true,
		"ActiveMenu": "password",
	}
	c.Render(w, r, "admin/password", data)
}
