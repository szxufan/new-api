package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/cachex"
	"github.com/samber/hot"
)

const (
	mcpImageCacheNamespace = "new-api:mcp_image:v1"
	mcpImageCacheTTL       = 30 * time.Minute
	mcpImageCacheCapacity  = 10_000
)

// MCPImageEntry 存储图片的二进制数据、MIME 类型和原始大小
type MCPImageEntry struct {
	Data     []byte `json:"data"`
	MimeType string `json:"mime_type"`
	OrigSize int64  `json:"orig_size"`
}

var (
	mcpImageCacheOnce sync.Once
	mcpImageCache     *cachex.HybridCache[MCPImageEntry]
)

func getMCPImageCache() *cachex.HybridCache[MCPImageEntry] {
	mcpImageCacheOnce.Do(func() {
		mcpImageCache = cachex.NewHybridCache[MCPImageEntry](cachex.HybridCacheConfig[MCPImageEntry]{
			Namespace: cachex.Namespace(mcpImageCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[MCPImageEntry]{},
			Memory: func() *hot.HotCache[string, MCPImageEntry] {
				return hot.NewHotCache[string, MCPImageEntry](hot.LRU, mcpImageCacheCapacity).
					WithTTL(mcpImageCacheTTL).
					WithJanitor().
					Build()
			},
		})
	})
	return mcpImageCache
}

// generateImageID 根据图片 URL 生成唯一 ID
func generateImageID(imageURL string) string {
	h := sha256.New()
	h.Write([]byte(imageURL))
	h.Write([]byte(time.Now().Format(time.RFC3339Nano)))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// DownloadAndCacheImage 从 URL 下载图片并存入缓存，返回缓存 ID
// proxyURL 可选，为空时不使用代理
func DownloadAndCacheImage(imageURL string, proxyURL string) (imageID string, mimeType string, err error) {
	client, err := GetHttpClientWithProxy(proxyURL)
	if err != nil {
		return "", "", fmt.Errorf("failed to create http client: %w", err)
	}

	resp, err := client.Get(imageURL)
	if err != nil {
		return "", "", fmt.Errorf("failed to download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("failed to download image, status: %d", resp.StatusCode)
	}

	// 限制最大 20MB
	const maxSize = 20 * 1024 * 1024
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxSize+1))
	if err != nil {
		return "", "", fmt.Errorf("failed to read image data: %w", err)
	}
	if len(data) > maxSize {
		return "", "", fmt.Errorf("image too large: %d bytes", len(data))
	}

	// 检测 MIME 类型
	mt := resp.Header.Get("Content-Type")
	if mt == "" {
		mt = http.DetectContentType(data)
	}

	imageID = generateImageID(imageURL)
	entry := MCPImageEntry{
		Data:     data,
		MimeType: mt,
		OrigSize: int64(len(data)),
	}

	cache := getMCPImageCache()
	if err := cache.SetWithTTL(imageID, entry, mcpImageCacheTTL); err != nil {
		return "", "", fmt.Errorf("failed to cache image: %w", err)
	}

	return imageID, mt, nil
}

// GetCachedImage 从缓存中获取图片数据
func GetCachedImage(imageID string) (*MCPImageEntry, bool) {
	cache := getMCPImageCache()
	entry, found, err := cache.Get(imageID)
	if err != nil || !found {
		return nil, false
	}
	return &entry, true
}

// SetCachedImage 将图片数据存入缓存
func SetCachedImage(imageID string, entry MCPImageEntry) error {
	cache := getMCPImageCache()
	return cache.SetWithTTL(imageID, entry, mcpImageCacheTTL)
}

// MimeTypeToExt 根据 MIME 类型返回文件扩展名（含点号）
func MimeTypeToExt(mimeType string) string {
	switch mimeType {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	case "image/svg+xml":
		return ".svg"
	case "image/tiff":
		return ".tiff"
	default:
		return ""
	}
}
