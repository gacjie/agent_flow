package service

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"agent_flow/src/common"
	"agent_flow/src/config"
	"agent_flow/src/model"

	"gorm.io/gorm"
)

type attemptInfo struct {
	Count    int
	LockedAt time.Time
}

// cachedSession Session 缓存条目
type cachedSession struct {
	Admin     *model.Admin
	IP        string
	ExpiresAt time.Time
	CachedAt  time.Time
}

// AuthService 认证业务逻辑（含 Session 缓存）
type AuthService struct {
	DB           *gorm.DB
	mu           sync.Mutex
	attempts     map[string]*attemptInfo
	cacheMu      sync.RWMutex
	sessionCache map[string]*cachedSession
}

// NewAuthService 创建认证服务实例
func NewAuthService(db *gorm.DB) *AuthService {
	return &AuthService{
		DB:           db,
		attempts:     make(map[string]*attemptInfo),
		sessionCache: make(map[string]*cachedSession),
	}
}

// Login 管理员登录（含暴力破解防护）
func (s *AuthService) Login(username, password, ip, userAgent string) (string, error) {
	s.mu.Lock()
	info := s.attempts[username]
	cfg := config.Get()
	lockDur := time.Duration(cfg.Auth.LockDuration) * time.Minute
	if info != nil && !info.LockedAt.IsZero() && time.Now().Before(info.LockedAt.Add(lockDur)) {
		remaining := time.Until(info.LockedAt.Add(lockDur)).Minutes()
		s.mu.Unlock()
		return "", common.NewError(http.StatusTooManyRequests,
			fmt.Sprintf("登录失败次数过多，请 %.0f 分钟后重试", remaining+1))
	}
	s.mu.Unlock()

	var admin model.Admin
	err := s.DB.Preload("Role").Where("username = ? AND deleted_at IS NULL", username).First(&admin).Error
	if err != nil {
		s.recordFailure(username, ip)
		return "", common.NewError(http.StatusUnauthorized, "用户名或密码错误")
	}

	if admin.Status != 1 {
		return "", common.NewError(http.StatusForbidden, "账号已被禁用")
	}

	if !common.CheckPassword(password, admin.Password) {
		s.recordFailure(username, ip)
		return "", common.NewError(http.StatusUnauthorized, "用户名或密码错误")
	}

	s.mu.Lock()
	delete(s.attempts, username)
	s.mu.Unlock()

	token, err := common.GenerateToken(common.SessionTokenLen)
	if err != nil {
		return "", common.NewError(http.StatusInternalServerError, "Token 生成失败")
	}

	cfg = config.Get()
	maxAge := cfg.Auth.SessionMaxAge
	if maxAge <= 0 {
		maxAge = 86400
	}

	expiresAt := time.Now().Add(time.Duration(maxAge) * time.Second)
	session := &model.Session{
		Token:     token,
		AdminID:   admin.ID,
		IP:        ip,
		UserAgent: common.TruncateString(userAgent, 255),
		ExpiresAt: expiresAt,
	}

	if err := s.DB.Create(session).Error; err != nil {
		return "", common.NewError(http.StatusInternalServerError, "Session 创建失败")
	}

	// 写入缓存
	s.cacheMu.Lock()
	s.sessionCache[token] = &cachedSession{
		Admin:     &admin,
		IP:        ip,
		ExpiresAt: expiresAt,
		CachedAt:  time.Now(),
	}
	s.cacheMu.Unlock()

	slog.Info("管理员登录", "username", username, "ip", ip)
	return token, nil
}

func (s *AuthService) recordFailure(username, ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	info := s.attempts[username]
	if info == nil {
		info = &attemptInfo{}
		s.attempts[username] = info
	}
	info.Count++
	if info.Count >= config.Get().Auth.MaxLoginAttempts {
		info.LockedAt = time.Now()
		slog.Warn("账号因多次登录失败被锁定", "username", username, "ip", ip)
	}
}

// Logout 管理员登出
func (s *AuthService) Logout(token string) {
	s.cacheMu.Lock()
	delete(s.sessionCache, token)
	s.cacheMu.Unlock()
	s.DB.Where("token = ?", token).Delete(&model.Session{})
}

// ValidateSession 验证 Session（优先查缓存，减少 DB 查询）
func (s *AuthService) ValidateSession(token, ip string) (*model.Admin, error) {
	if token == "" {
		return nil, common.ErrUnauthorized
	}

	// 1. 查缓存
	s.cacheMu.RLock()
	cached, ok := s.sessionCache[token]
	s.cacheMu.RUnlock()

	cacheTTL := time.Duration(config.Get().Auth.SessionCacheTTL) * time.Second
	if ok && time.Since(cached.CachedAt) < cacheTTL {
		if time.Now().After(cached.ExpiresAt) {
			s.invalidateSession(token)
			return nil, common.ErrUnauthorized
		}
		sessionHost := extractIP(cached.IP)
		requestHost := extractIP(ip)
		if sessionHost != "" && requestHost != "" && sessionHost != requestHost {
			slog.Warn("Session IP 不匹配", "session_ip", sessionHost, "request_ip", requestHost)
			s.invalidateSession(token)
			return nil, common.ErrUnauthorized
		}
		if cached.Admin.Status != 1 {
			return nil, common.NewError(http.StatusForbidden, "账号已被禁用")
		}
		return cached.Admin, nil
	}

	// 2. 缓存未命中，查 DB
	var session model.Session
	if err := s.DB.Where("token = ?", token).First(&session).Error; err != nil {
		return nil, common.ErrUnauthorized
	}

	if session.IsExpired() {
		s.DB.Delete(&session)
		return nil, common.ErrUnauthorized
	}

	sessionHost := extractIP(session.IP)
	requestHost := extractIP(ip)
	if sessionHost != "" && requestHost != "" && sessionHost != requestHost {
		slog.Warn("Session IP 不匹配", "session_ip", sessionHost, "request_ip", requestHost)
		s.DB.Delete(&session)
		return nil, common.ErrUnauthorized
	}

	var admin model.Admin
	if err := s.DB.Preload("Role").First(&admin, session.AdminID).Error; err != nil {
		return nil, common.ErrUnauthorized
	}

	if admin.Status != 1 {
		return nil, common.NewError(http.StatusForbidden, "账号已被禁用")
	}

	// 写入缓存
	s.cacheMu.Lock()
	s.sessionCache[token] = &cachedSession{
		Admin:     &admin,
		IP:        session.IP,
		ExpiresAt: session.ExpiresAt,
		CachedAt:  time.Now(),
	}
	s.cacheMu.Unlock()

	return &admin, nil
}

// invalidateSession 清除缓存和数据库中的 Session
func (s *AuthService) invalidateSession(token string) {
	s.cacheMu.Lock()
	delete(s.sessionCache, token)
	s.cacheMu.Unlock()
	s.DB.Where("token = ?", token).Delete(&model.Session{})
}

func extractIP(addr string) string {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

// CleanExpiredSessions 清理过期 Session
func (s *AuthService) CleanExpiredSessions() {
	result := s.DB.Where("expires_at < ?", time.Now()).Delete(&model.Session{})
	if result.RowsAffected > 0 {
		slog.Info("清理过期 Session", "count", result.RowsAffected)
	}
}
