package browsers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

const MaxEventLogs = 20
const cookieFileName = "cookies.json"

type Session struct {
	Browser             *rod.Browser
	Page                *rod.Page
	UserDataDir         string
	LastUsed            time.Time
	ConsoleLogs         []string
	NetworkEvents       []string
	LastNavURL          string
	LastNavStatus       int64
	LastNavMimeType     string
	LastInteractionRefs []InteractionRef
	LastRefURL          string
	Mu                  sync.Mutex
}

type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	once     sync.Once
}

var Mgr = &Manager{
	sessions: make(map[string]*Session),
}

func (m *Manager) StartCleaner() {
	m.once.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			for range ticker.C {
				m.Cleanup()
			}
		}()
	})
}

func (m *Manager) Cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for key, sess := range m.sessions {
		sess.Mu.Lock()
		idle := time.Since(sess.LastUsed) > 5*time.Minute
		sess.Mu.Unlock()
		if idle {
			slog.Info("浏览器会话空闲超时，自动关闭", "key", key)
			saveCookies(sess)
			sess.Browser.MustClose()
			delete(m.sessions, key)
		}
	}
}

func (m *Manager) GetOrCreate(key string, width, height int, workDir string) (*Session, error) {
	m.StartCleaner()
	m.mu.Lock()
	defer m.mu.Unlock()

	if sess, ok := m.sessions[key]; ok {
		sess.Mu.Lock()
		sess.LastUsed = time.Now()
		sess.Mu.Unlock()
		return sess, nil
	}

	// 浏览器查找策略：本地已安装 > 应用目录下载（所有工作区共享）
	var browserPath string
	if path, found := launcher.LookPath(); found {
		browserPath = path
		slog.Info("使用本地浏览器", "path", path)
	} else {
		slog.Info("本地未找到浏览器，将自动下载 Chromium")
		exe, _ := os.Executable()
		cacheDir := filepath.Join(filepath.Dir(exe), "browser")
		os.MkdirAll(cacheDir, 0755)
		b := launcher.NewBrowser()
		b.RootDir = cacheDir
		p, err := b.Get()
		if err != nil {
			return nil, fmt.Errorf("下载 Chromium 失败: %w", err)
		}
		browserPath = p
		slog.Info("Chromium 已就绪", "path", p)
	}

	var userDataDir string
	if workDir != "" {
		userDataDir = filepath.Join(workDir, "browser")
	} else {
		userDataDir = filepath.Join(os.TempDir(), "rod-"+key)
	}
	os.MkdirAll(userDataDir, 0755)

	l := launcher.New().
		Bin(browserPath).
		NoSandbox(true).
		Headless(true).
		UserDataDir(userDataDir).
		Set("disable-gpu").
		Set("disable-dev-shm-usage").
		Set("disable-features", "SameSiteByDefaultCookies,CookiesWithoutSameSiteMustBeSecure").
		Set("allow-insecure-localhost").
		Env(append(os.Environ(), "HOME="+userDataDir)...)

	u, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("浏览器启动失败: %w", err)
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return nil, fmt.Errorf("浏览器连接失败: %w", err)
	}

	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		browser.MustClose()
		return nil, fmt.Errorf("创建页面失败: %w", err)
	}

	if err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
		Width: width, Height: height, DeviceScaleFactor: 1,
	}); err != nil {
		browser.MustClose()
		return nil, fmt.Errorf("设置视口失败: %w", err)
	}

	sess := &Session{
		Browser:     browser,
		Page:        page,
		UserDataDir: userDataDir,
		LastUsed:    time.Now(),
	}

	// 事件监听：console + network
	go page.EachEvent(
		func(e *proto.RuntimeConsoleAPICalled) {
			if e.Type != proto.RuntimeConsoleAPICalledTypeError && e.Type != proto.RuntimeConsoleAPICalledTypeWarning {
				return
			}
			var parts []string
			for _, arg := range e.Args {
				val := arg.Value.String()
				if val == "" && arg.Description != "" {
					val = arg.Description
				}
				if val != "" {
					parts = append(parts, strings.Trim(val, `"`))
				}
			}
			if len(parts) > 0 {
				sess.Mu.Lock()
				AppendBoundedLog(&sess.ConsoleLogs, fmt.Sprintf("[%s] %s", e.Type, strings.Join(parts, " ")))
				sess.Mu.Unlock()
			}
		},
		func(e *proto.RuntimeExceptionThrown) {
			sess.Mu.Lock()
			AppendBoundedLog(&sess.ConsoleLogs, fmt.Sprintf("[exception] %s", e.ExceptionDetails.Text))
			sess.Mu.Unlock()
		},
		func(e *proto.NetworkRequestWillBeSent) {
			if e.Type != proto.NetworkResourceTypeDocument {
				return
			}
			sess.Mu.Lock()
			sess.LastNavURL = e.Request.URL
			AppendBoundedLog(&sess.NetworkEvents, fmt.Sprintf("[request] %s", e.Request.URL))
			sess.Mu.Unlock()
		},
		func(e *proto.NetworkResponseReceived) {
			if e.Type != proto.NetworkResourceTypeDocument {
				return
			}
			sess.Mu.Lock()
			sess.LastNavURL = e.Response.URL
			sess.LastNavStatus = int64(e.Response.Status)
			sess.LastNavMimeType = e.Response.MIMEType
			AppendBoundedLog(&sess.NetworkEvents, fmt.Sprintf("[response] %d %s (%s)", e.Response.Status, e.Response.URL, e.Response.MIMEType))
			sess.Mu.Unlock()
		},
		func(e *proto.NetworkLoadingFailed) {
			sess.Mu.Lock()
			AppendBoundedLog(&sess.NetworkEvents, fmt.Sprintf("[loading_failed] %s", strings.TrimSpace(e.ErrorText)))
			sess.Mu.Unlock()
		},
	)()

	restoreCookies(sess)
	m.sessions[key] = sess
	return sess, nil
}

func (m *Manager) Get(key string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sess, ok := m.sessions[key]; ok {
		sess.Mu.Lock()
		sess.LastUsed = time.Now()
		sess.Mu.Unlock()
		return sess
	}
	return nil
}

func (m *Manager) Close(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if sess, ok := m.sessions[key]; ok {
		saveCookies(sess)
		sess.Browser.MustClose()
		delete(m.sessions, key)
	}
}

func AppendBoundedLog(logs *[]string, msg string) {
	if msg == "" {
		return
	}
	*logs = append(*logs, msg)
	if len(*logs) > MaxEventLogs {
		*logs = (*logs)[len(*logs)-MaxEventLogs:]
	}
}

func CloneLogs(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	out := make([]string, len(src))
	copy(out, src)
	return out
}

func saveCookies(sess *Session) {
	if sess.UserDataDir == "" || sess.Page == nil {
		return
	}
	res, err := proto.NetworkGetAllCookies{}.Call(sess.Page)
	if err != nil {
		slog.Warn("保存 cookies 失败", "error", err)
		return
	}
	if len(res.Cookies) == 0 {
		return
	}
	data, err := json.Marshal(res.Cookies)
	if err != nil {
		slog.Warn("保存 cookies 序列化失败", "error", err)
		return
	}
	path := filepath.Join(sess.UserDataDir, cookieFileName)
	if err := os.WriteFile(path, data, 0600); err != nil {
		slog.Warn("保存 cookies 写入失败", "error", err, "path", path)
	}
}

func restoreCookies(sess *Session) {
	if sess.UserDataDir == "" || sess.Page == nil {
		return
	}
	path := filepath.Join(sess.UserDataDir, cookieFileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var cookies []*proto.NetworkCookie
	if err := json.Unmarshal(data, &cookies); err != nil {
		slog.Warn("恢复 cookies 解析失败", "error", err)
		os.Remove(path)
		return
	}
	if len(cookies) == 0 {
		os.Remove(path)
		return
	}
	var params []*proto.NetworkCookieParam
	for _, c := range cookies {
		p := &proto.NetworkCookieParam{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Secure:   c.Secure,
			HTTPOnly: c.HTTPOnly,
			SameSite: c.SameSite,
		}
		if c.Expires > 0 {
			p.Expires = proto.TimeSinceEpoch(c.Expires)
		}
		params = append(params, p)
	}
	if err := (proto.NetworkSetCookies{Cookies: params}).Call(sess.Page); err != nil {
		slog.Warn("恢复 cookies 失败", "error", err)
	} else {
		os.Remove(path)
	}
}
