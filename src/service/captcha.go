package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	mrand "math/rand/v2"
	"strings"
	"sync"
	"time"

	"agent_flow/src/config"
)

const (
	captchaCleanup   = 2 * time.Minute
	captchaSVGWidth  = 130
	captchaSVGHeight = 40
)

type captchaItem struct {
	Answer    int
	ExpiresAt time.Time
}

// CaptchaService 算术验证码服务（内存存储）
type CaptchaService struct {
	mu    sync.RWMutex
	store map[string]*captchaItem
}

// NewCaptchaService 创建验证码服务并启动过期清理
func NewCaptchaService() *CaptchaService {
	s := &CaptchaService{store: make(map[string]*captchaItem)}
	go s.cleanup()
	return s
}

// Generate 生成验证码，返回 (ID, SVG HTML)
func (s *CaptchaService) Generate() (string, string) {
	a := mrand.IntN(20) + 1
	b := mrand.IntN(20) + 1
	op := "+"
	answer := a + b

	if mrand.IntN(2) == 0 && a > b {
		op = "-"
		answer = a - b
	}

	expr := fmt.Sprintf("%d %s %d = ?", a, op, b)

	id := generateCaptchaID()
	s.mu.Lock()
	s.store[id] = &captchaItem{Answer: answer, ExpiresAt: time.Now().Add(time.Duration(config.Get().Auth.CaptchaTTL) * time.Second)}
	s.mu.Unlock()

	svg := renderSVG(expr)
	return id, svg
}

// Verify 校验验证码（一次性，验证后删除）
func (s *CaptchaService) Verify(id string, answer int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.store[id]
	if !ok {
		return false
	}
	delete(s.store, id)

	if time.Now().After(item.ExpiresAt) {
		return false
	}
	return item.Answer == answer
}

// GenerateJSON 生成验证码并返回 JSON 友好的结果
func (s *CaptchaService) GenerateJSON() map[string]string {
	id, svg := s.Generate()
	return map[string]string{"id": id, "svg": svg}
}

func (s *CaptchaService) cleanup() {
	ticker := time.NewTicker(captchaCleanup)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		now := time.Now()
		for id, item := range s.store {
			if now.After(item.ExpiresAt) {
				delete(s.store, id)
			}
		}
		s.mu.Unlock()
	}
}

func generateCaptchaID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// renderSVG 生成算术表达式的 SVG 图片
func renderSVG(expr string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`,
		captchaSVGWidth, captchaSVGHeight, captchaSVGWidth, captchaSVGHeight,
	))

	// 干扰线（3条随机线条）
	for i := 0; i < 3; i++ {
		x1 := mrand.IntN(captchaSVGWidth)
		y1 := mrand.IntN(captchaSVGHeight)
		x2 := mrand.IntN(captchaSVGWidth)
		y2 := mrand.IntN(captchaSVGHeight)
		opacity := randFloat(0.15, 0.3)
		b.WriteString(fmt.Sprintf(
			`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="currentColor" stroke-width="1" opacity="%.2f"/>`,
			x1, y1, x2, y2, opacity,
		))
	}

	// 干扰点（8个随机圆点）
	for i := 0; i < 8; i++ {
		cx := mrand.IntN(captchaSVGWidth)
		cy := mrand.IntN(captchaSVGHeight)
		r := randFloat(1, 2.5)
		opacity := randFloat(0.15, 0.35)
		b.WriteString(fmt.Sprintf(
			`<circle cx="%d" cy="%d" r="%.1f" fill="currentColor" opacity="%.2f"/>`,
			cx, cy, r, opacity,
		))
	}

	// 逐字符绘制，随机偏移
	startX := 10
	for i, ch := range expr {
		x := startX + i*15
		y := 26 + mrand.IntN(7) - 3
		fontSize := 18 + mrand.IntN(5) - 2
		rotation := mrand.IntN(15) - 7
		b.WriteString(fmt.Sprintf(
			`<text x="%d" y="%d" font-size="%d" font-family="monospace" fill="currentColor" transform="rotate(%d %d %d)">%s</text>`,
			x, y, fontSize, rotation, x, y, string(ch),
		))
	}

	b.WriteString(`</svg>`)
	return b.String()
}

func randFloat(min, max float64) float64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(100))
	return min + (max-min)*float64(n.Int64())/100.0
}
