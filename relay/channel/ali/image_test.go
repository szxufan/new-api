package ali

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 1x1 像素 PNG，用于构造裸 base64 输入
const tinyPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="

func mustImageRequest(t *testing.T, body string) dto.ImageRequest {
	t.Helper()
	var request dto.ImageRequest
	require.NoError(t, common.Unmarshal([]byte(body), &request))
	return request
}

func newTestRelayInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelBaseUrl: "https://dashscope.aliyuncs.com",
		},
	}
}

// contentOf 取出转换结果里 input.messages[0].content，便于断言
func contentOf(t *testing.T, converted *AliImageRequest) []AliMediaContent {
	t.Helper()
	input, ok := converted.Input.(AliImageInput)
	require.True(t, ok, "input should be AliImageInput")
	require.Len(t, input.Messages, 1)
	content, ok := input.Messages[0].Content.([]AliMediaContent)
	require.True(t, ok, "message content should be []AliMediaContent")
	return content
}

func TestOaiImage2AliImageRequest_JSONMultiImageEdit(t *testing.T) {
	// 用户实际请求形态：/v1/images/edits + application/json + image 为 data URL 数组
	request := mustImageRequest(t, `{
		"model": "qwen-image-3.0",
		"prompt": "put the cat on the sofa",
		"image": ["data:image/png;base64,AAA", "data:image/jpeg;base64,BBB"],
		"response_format": "b64_json"
	}`)

	converted, err := oaiImage2AliImageRequest(newTestRelayInfo(), request, true)
	require.NoError(t, err)

	content := contentOf(t, converted)
	require.Len(t, content, 3, "两张输入图 + 一条文本指令")
	assert.Equal(t, "data:image/png;base64,AAA", content[0].Image, "数组顺序即图像顺序")
	assert.Equal(t, "data:image/jpeg;base64,BBB", content[1].Image)
	assert.Equal(t, "put the cat on the sofa", content[2].Text)
	assert.Empty(t, content[2].Image)
}

func TestOaiImage2AliImageRequest_JSONSingleImageString(t *testing.T) {
	request := mustImageRequest(t, `{
		"model": "qwen-image-3.0-pro",
		"prompt": "make it snowy",
		"image": "https://example.com/source.png"
	}`)

	converted, err := oaiImage2AliImageRequest(newTestRelayInfo(), request, true)
	require.NoError(t, err)

	content := contentOf(t, converted)
	require.Len(t, content, 2)
	assert.Equal(t, "https://example.com/source.png", content[0].Image, "公网 URL 原样透传")
	assert.Equal(t, "make it snowy", content[1].Text)
}

func TestOaiImage2AliImageRequest_JSONImagesField(t *testing.T) {
	request := mustImageRequest(t, `{
		"model": "qwen-image-edit-plus",
		"prompt": "merge them",
		"images": ["data:image/png;base64,AAA", "data:image/png;base64,BBB"]
	}`)

	converted, err := oaiImage2AliImageRequest(newTestRelayInfo(), request, true)
	require.NoError(t, err)

	content := contentOf(t, converted)
	require.Len(t, content, 3)
	assert.Equal(t, "data:image/png;base64,AAA", content[0].Image)
	assert.Equal(t, "data:image/png;base64,BBB", content[1].Image)
}

func TestOaiImage2AliImageRequest_RawBase64NormalizedToDataURL(t *testing.T) {
	request := mustImageRequest(t, `{
		"model": "qwen-image-3.0",
		"prompt": "brighten the photo",
		"image": "`+tinyPNGBase64+`"
	}`)

	converted, err := oaiImage2AliImageRequest(newTestRelayInfo(), request, true)
	require.NoError(t, err)

	content := contentOf(t, converted)
	require.Len(t, content, 2)
	assert.Equal(t, "data:image/png;base64,"+tinyPNGBase64, content[0].Image,
		"裸 base64 必须补全为 DashScope 要求的 data:{MIME};base64,{data} 格式")
}

func TestOaiImage2AliImageRequest_TextToImageKeepsTextOnly(t *testing.T) {
	request := mustImageRequest(t, `{
		"model": "qwen-image-3.0",
		"prompt": "a red apple on a table"
	}`)

	converted, err := oaiImage2AliImageRequest(newTestRelayInfo(), request, true)
	require.NoError(t, err)

	content := contentOf(t, converted)
	require.Len(t, content, 1, "文生图只包含一个 text 对象")
	assert.Equal(t, "a red apple on a table", content[0].Text)
	assert.Empty(t, content[0].Image)
}

func TestOaiImage2AliImageRequest_ExplicitInputNotOverwritten(t *testing.T) {
	// 客户端直接传阿里原生 input 时，不应被 OpenAI 字段的转换结果覆盖
	request := mustImageRequest(t, `{
		"model": "qwen-image-3.0",
		"prompt": "ignored",
		"image": ["data:image/png;base64,AAA"],
		"input": {"messages": [{"role": "user", "content": [{"image": "https://example.com/native.png"}, {"text": "native prompt"}]}]}
	}`)

	converted, err := oaiImage2AliImageRequest(newTestRelayInfo(), request, true)
	require.NoError(t, err)

	body, err := common.Marshal(converted)
	require.NoError(t, err)
	assert.Contains(t, string(body), "https://example.com/native.png")
	assert.Contains(t, string(body), "native prompt")
	assert.NotContains(t, string(body), "data:image/png;base64,AAA")
	assert.NotContains(t, string(body), "ignored")
}

func TestOaiImage2AliImageRequest_QwenImageRejectsTooManyImages(t *testing.T) {
	request := mustImageRequest(t, `{
		"model": "qwen-image-3.0",
		"prompt": "four images",
		"image": ["data:image/png;base64,A", "data:image/png;base64,B", "data:image/png;base64,C", "data:image/png;base64,D"]
	}`)

	_, err := oaiImage2AliImageRequest(newTestRelayInfo(), request, true)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at most 3 input images")
}

func TestOaiImage2AliImageRequest_AllowsThreeImages(t *testing.T) {
	request := mustImageRequest(t, `{
		"model": "qwen-image-3.0-pro",
		"prompt": "three images",
		"image": ["data:image/png;base64,A", "data:image/png;base64,B", "data:image/png;base64,C"]
	}`)

	converted, err := oaiImage2AliImageRequest(newTestRelayInfo(), request, true)
	require.NoError(t, err)
	assert.Len(t, contentOf(t, converted), 4)
}

func TestOaiImage2AliImageRequest_DoesNotPassthroughResponseFormat(t *testing.T) {
	request := mustImageRequest(t, `{
		"model": "qwen-image-3.0",
		"prompt": "hello",
		"response_format": "b64_json"
	}`)

	converted, err := oaiImage2AliImageRequest(newTestRelayInfo(), request, true)
	require.NoError(t, err)

	body, err := common.Marshal(converted)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "response_format",
		"DashScope 不接受 response_format，透传会导致上游 400")
}

func TestOaiImage2AliImageRequest_QwenImage3ParametersPassthrough(t *testing.T) {
	request := mustImageRequest(t, `{
		"model": "qwen-image-3.0",
		"prompt": "hello",
		"parameters": {"prompt_extend": true, "prompt_extend_mode": "direct", "enable_thinking": false, "n": 2, "size": "1024*1024"}
	}`)

	converted, err := oaiImage2AliImageRequest(newTestRelayInfo(), request, true)
	require.NoError(t, err)

	assert.Equal(t, "direct", converted.Parameters.PromptExtendMode)
	require.NotNil(t, converted.Parameters.EnableThinking)
	assert.False(t, *converted.Parameters.EnableThinking)
	require.NotNil(t, converted.Parameters.PromptExtend)
	assert.True(t, *converted.Parameters.PromptExtend)
	assert.Equal(t, 2, converted.Parameters.N)
}

func TestOaiImage2AliImageRequest_AsyncKeepsPromptField(t *testing.T) {
	request := mustImageRequest(t, `{
		"model": "qwen-image-plus",
		"prompt": "async text to image",
		"size": "1024x1024"
	}`)

	converted, err := oaiImage2AliImageRequest(newTestRelayInfo(), request, false)
	require.NoError(t, err)

	input, ok := converted.Input.(AliImageInput)
	require.True(t, ok)
	assert.Equal(t, "async text to image", input.Prompt)
	assert.Empty(t, input.Messages)
	assert.Equal(t, "1024*1024", converted.Parameters.Size, "size 需转换为宽*高格式")
}

// newMultipartEditContext 构造只带 image 文件字段的 multipart 请求上下文，
// 文本字段由 dto.ImageRequest 直接传入（表单文本字段的解析在 valid_request 层完成）。
func newMultipartEditContext(t *testing.T, images [][]byte) *gin.Context {
	t.Helper()
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for i, data := range images {
		part, err := writer.CreateFormFile("image", fmt.Sprintf("image-%d.png", i))
		require.NoError(t, err)
		_, err = part.Write(data)
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/images/edits", body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	return c
}

func TestOaiFormEdit2AliImageEdit_MultipleFiles(t *testing.T) {
	png, err := base64.StdEncoding.DecodeString(tinyPNGBase64)
	require.NoError(t, err)

	c := newMultipartEditContext(t, [][]byte{png, png})
	request := dto.ImageRequest{
		Model:  "qwen-image-3.0",
		Prompt: "edit both images",
		Size:   "1024x1024",
	}

	converted, err := oaiFormEdit2AliImageEdit(c, newTestRelayInfo(), request)
	require.NoError(t, err)

	content := contentOf(t, converted)
	require.Len(t, content, 3, "两个表单文件 + 一条文本指令")
	assert.True(t, strings.HasPrefix(content[0].Image, "data:image/png;base64,"))
	assert.True(t, strings.HasPrefix(content[1].Image, "data:image/png;base64,"))
	assert.Equal(t, "edit both images", content[2].Text)
	assert.Equal(t, "1024*1024", converted.Parameters.Size)

	body, err := common.Marshal(converted)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "response_format")
}

func TestNormalizeImageInputs(t *testing.T) {
	images, err := normalizeImageInputs([]string{
		"",
		"   ",
		"https://example.com/a.png",
		"data:image/webp;base64,AAA",
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"https://example.com/a.png", "data:image/webp;base64,AAA"}, images)

	empty, err := normalizeImageInputs([]string{"", "  "})
	require.NoError(t, err)
	assert.Empty(t, empty)

	_, err = normalizeImageInputs([]string{"not-base64-!!!"})
	require.Error(t, err)
}

func TestIsQwenImageModel(t *testing.T) {
	for _, model := range []string{"qwen-image", "qwen-image-3.0", "qwen-image-3.0-pro", "qwen-image-edit-plus", "QWEN-IMAGE-EDIT-MAX"} {
		assert.True(t, isQwenImageModel(model), model)
	}
	for _, model := range []string{"wan2.7-image", "z-image-turbo", "dall-e-3"} {
		assert.False(t, isQwenImageModel(model), model)
	}
}
