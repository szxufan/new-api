package gemini

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/gin-gonic/gin"
)

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

// newMultipartContext 构造携带 input_reference 文件的 multipart 测试上下文。
func newMultipartContext(t *testing.T, fileName string, size int) (*gin.Context, *relaycommon.RelayInfo) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	fw, err := w.CreateFormFile("input_reference", fileName)
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := io.Copy(fw, io.LimitReader(zeroReader{}, int64(size))); err != nil {
		t.Fatalf("copy file content: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", body)
	c.Request.Header.Set("Content-Type", w.FormDataContentType())

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "veo-3.0-generate-preview",
		},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{
			PublicTaskID: "task_public_0001",
			Action:       constant.TaskActionTextGenerate,
		},
	}
	return c, info
}

func TestExtractMultipartImageNoFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"prompt":"p"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	info := &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: constant.TaskActionTextGenerate},
	}

	img, err := ExtractMultipartImage(c, info)
	if err != nil {
		t.Fatalf("ExtractMultipartImage() error = %v, want nil", err)
	}
	if img != nil {
		t.Fatalf("ExtractMultipartImage() = %+v, want nil", img)
	}
	if info.Action != constant.TaskActionTextGenerate {
		t.Errorf("info.Action = %q, want textGenerate (unchanged)", info.Action)
	}
}

func TestExtractMultipartImageSmallFile(t *testing.T) {
	c, info := newMultipartContext(t, "a.png", 1024)

	img, err := ExtractMultipartImage(c, info)
	if err != nil {
		t.Fatalf("ExtractMultipartImage() error = %v", err)
	}
	if img == nil || img.BytesBase64Encoded == "" {
		t.Fatalf("ExtractMultipartImage() = %+v, want non-empty image", img)
	}
	if info.Action != constant.TaskActionGenerate {
		t.Errorf("info.Action = %q, want generate", info.Action)
	}
}

// TestExtractMultipartImageOversized 锚定 D5 修复：
// 超过 20MB 的上传文件必须返回明确错误，而不是静默退化为文生视频。
func TestExtractMultipartImageOversized(t *testing.T) {
	c, info := newMultipartContext(t, "big.png", maxVeoImageSize+1)

	img, err := ExtractMultipartImage(c, info)
	if err == nil {
		t.Fatalf("ExtractMultipartImage() expected error for >20MB file, got nil (img=%+v)", img)
	}
	if !strings.Contains(err.Error(), "size limit") {
		t.Errorf("error = %q, want mention of size limit", err.Error())
	}
	if info.Action != constant.TaskActionTextGenerate {
		t.Errorf("info.Action = %q, want textGenerate (unchanged on error)", info.Action)
	}
}
