package tool

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agent_flow/src/common"
)

// ImageGenTool 图片生成工具（支持批量）
type ImageGenTool struct {
	ConfigGetter ConfigGetter
	ModelGetter  ModelGetter
}

// ConfigGetter 系统配置读取接口（避免循环依赖）
type ConfigGetter interface {
	Get(key string) string
}

// ModelGetter 模型信息获取接口（避免循环依赖）
type ModelGetter interface {
	GetModelForImageGen() (baseURL, apiKey, apiModelID, protocol string, err error)
}

func (t *ImageGenTool) Name() string { return "generate_images" }

func (t *ImageGenTool) Description() string {
	return "根据文本描述生成图片。支持批量生成。可通过 save_path 指定保存到项目目录的路径，未指定时保存到工作区 uploads/ 目录。"
}

func (t *ImageGenTool) Parameters() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"prompt": {"type": "string", "description": "图片描述（单次生成时使用）"},
			"size": {"type": "string", "description": "图片尺寸，如 1024x1024、1792x1024、1024x1792（默认 1024x1024）"},
			"quality": {"type": "string", "description": "图片质量：standard 或 hd（默认 standard）"},
			"n": {"type": "integer", "description": "生成数量（默认 1，最大 4）"},
			"save_path": {"type": "string", "description": "保存路径（相对于项目目录），如 assets/images/hero.png。未指定时保存到工作区 uploads/ 目录"},
			"images": {
				"type": "array",
				"description": "批量生成列表（存在时优先使用）",
				"items": {
					"type": "object",
					"properties": {
						"prompt": {"type": "string", "description": "图片描述"},
						"size": {"type": "string"},
						"quality": {"type": "string"},
						"n": {"type": "integer"},
						"save_path": {"type": "string", "description": "保存路径（相对于项目目录）"}
					},
					"required": ["prompt"]
				}
			}
		}
	}`)
}

// PLACEHOLDER_EXECUTE

func (t *ImageGenTool) Execute(ctx context.Context, args string) *Result {
	var params struct {
		Prompt   string `json:"prompt"`
		Size     string `json:"size"`
		Quality  string `json:"quality"`
		N        int    `json:"n"`
		SavePath string `json:"save_path"`
		Images   []struct {
			Prompt   string `json:"prompt"`
			Size     string `json:"size"`
			Quality  string `json:"quality"`
			N        int    `json:"n"`
			SavePath string `json:"save_path"`
		} `json:"images"`
	}
	if err := json.Unmarshal([]byte(args), &params); err != nil {
		return ErrorResult("参数解析失败: " + err.Error())
	}

	wc := GetWorkContext(ctx)
	if wc == nil {
		return ErrorResult("未设置工作目录上下文")
	}

	baseURL, apiKey, apiModelID, protocol, err := t.ModelGetter.GetModelForImageGen()
	if err != nil {
		return ErrorResult("获取图片生成模型配置失败: " + err.Error())
	}

	// 批量模式
	if len(params.Images) > 0 {
		var allImages []ImageData
		var results []string
		success, fail := 0, 0
		for i, item := range params.Images {
			size := item.Size
			if size == "" {
				size = "1024x1024"
			}
			quality := item.Quality
			if quality == "" {
				quality = "standard"
			}
			n := item.N
			if n <= 0 {
				n = 1
			}
			imgs, err := t.generateOne(ctx, baseURL, apiKey, apiModelID, protocol, item.Prompt, size, quality, n, item.SavePath)
			if err != nil {
				fail++
				results = append(results, fmt.Sprintf("✗ [%d] %s", i+1, err.Error()))
			} else {
				success++
				allImages = append(allImages, imgs...)
				results = append(results, fmt.Sprintf("✓ [%d] 生成 %d 张图片", i+1, len(imgs)))
			}
		}
		result := SuccessResult(fmt.Sprintf("批量生成结果 (%d 项，%d 成功 / %d 失败)：\n%s",
			len(params.Images), success, fail, strings.Join(results, "\n")))
		result.Images = allImages
		if fail > 0 && success == 0 {
			result.IsError = true
		}
		return result
	}

	// 单次模式
	if params.Prompt == "" {
		return ErrorResult("prompt 不能为空")
	}
	size := params.Size
	if size == "" {
		size = "1024x1024"
	}
	quality := params.Quality
	if quality == "" {
		quality = "standard"
	}
	n := params.N
	if n <= 0 {
		n = 1
	}

	imgs, err := t.generateOne(ctx, baseURL, apiKey, apiModelID, protocol, params.Prompt, size, quality, n, params.SavePath)
	if err != nil {
		return ErrorResult(err.Error())
	}

	result := SuccessResult(fmt.Sprintf("已生成 %d 张图片", len(imgs)))
	result.Images = imgs
	return result
}

func (t *ImageGenTool) generateOne(ctx context.Context, baseURL, apiKey, apiModelID, protocol, prompt, size, quality string, n int, savePath string) ([]ImageData, error) {
	switch protocol {
	case "openai", "openai-response", "":
		return t.generateOpenAI(ctx, baseURL, apiKey, apiModelID, prompt, size, quality, n, savePath)
	case "gemini":
		return nil, fmt.Errorf("Gemini 协议图片生成暂未实现")
	case "anthropic":
		return nil, fmt.Errorf("Anthropic 协议不支持图片生成")
	default:
		return t.generateOpenAI(ctx, baseURL, apiKey, apiModelID, prompt, size, quality, n, savePath)
	}
}

func (t *ImageGenTool) generateOpenAI(ctx context.Context, baseURL, apiKey, apiModelID, prompt, size, quality string, n int, savePath string) ([]ImageData, error) {
	url := common.BuildAPIURL(baseURL, "/images/generations")

	reqBody := map[string]interface{}{
		"model":           apiModelID,
		"prompt":          prompt,
		"size":            size,
		"quality":         quality,
		"n":               n,
		"response_format": "b64_json",
	}
	body, _ := json.Marshal(reqBody)

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		preview := string(respData)
		if len(preview) > 200 {
			preview = preview[:200]
		}
		return nil, fmt.Errorf("API 错误 %d: %s", resp.StatusCode, preview)
	}

	var result struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respData, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	// 保存图片
	wc := GetWorkContext(ctx)
	var images []ImageData
	for i, item := range result.Data {
		imgData, err := base64.StdEncoding.DecodeString(item.B64JSON)
		if err != nil {
			continue
		}

		var finalPath, relPath string
		if savePath != "" {
			sp := savePath
			if filepath.Ext(sp) == "" {
				sp += ".png"
			}
			if n > 1 && i > 0 {
				ext := filepath.Ext(sp)
				base := strings.TrimSuffix(sp, ext)
				sp = fmt.Sprintf("%s_%d%s", base, i, ext)
			}
			fullPath, err := safePath(ctx, sp)
			if err != nil {
				return nil, fmt.Errorf("save_path 路径校验失败: %w", err)
			}
			finalPath = fullPath
			relPath = sp
		} else {
			uploadDir := filepath.Join(wc.WorkspaceDir, "uploads")
			os.MkdirAll(uploadDir, 0755)
			filename := fmt.Sprintf("gen_%s_%d.png", time.Now().Format("20060102150405"), i)
			finalPath = filepath.Join(uploadDir, filename)
			relPath = "uploads/" + filename
		}

		os.MkdirAll(filepath.Dir(finalPath), 0755)
		if err := os.WriteFile(finalPath, imgData, 0644); err != nil {
			continue
		}
		images = append(images, ImageData{
			Path:      relPath,
			MediaType: "image/png",
			Data:      item.B64JSON,
		})
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("未能生成任何图片")
	}
	return images, nil
}