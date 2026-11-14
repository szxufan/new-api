package controller

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"math"
	"mime"
	"mime/multipart"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

// expectedFingerprintFloat 按 computeEmbeddingFingerprint 的算法约定独立复算期望指纹：
// 每个 float64 → math.Float64bits → 小端序 8 字节拼接 → MD5 hex。
func expectedFingerprintFloat(t *testing.T, vecs ...[]float64) string {
	t.Helper()
	buf := make([]byte, 0, len(vecs)*8)
	for _, v := range vecs {
		for _, f := range v {
			var b [8]byte
			binary.LittleEndian.PutUint64(b[:], math.Float64bits(f))
			buf = append(buf, b[:]...)
		}
	}
	sum := md5.Sum(buf)
	return bytes2hex(sum[:])
}

// expectedFingerprintStrings 按 base64 字符串路径算法约定复算期望指纹：
// 每个字符串后追加 0x00 分隔符 → 拼接 → MD5 hex。
func expectedFingerprintStrings(t *testing.T, strs ...string) string {
	t.Helper()
	var buf bytes.Buffer
	for _, s := range strs {
		buf.WriteString(s)
		buf.WriteByte(0x00)
	}
	sum := md5.Sum(buf.Bytes())
	return bytes2hex(sum[:])
}

func bytes2hex(b []byte) string {
	const hexChars = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexChars[v>>4]
		out[i*2+1] = hexChars[v&0x0f]
	}
	return string(out)
}

func TestComputeEmbeddingFingerprint_FloatArray(t *testing.T) {
	resp := `{"object":"list","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2,0.3]}],"model":"text-embedding-3-small","usage":{"prompt_tokens":2,"total_tokens":2}}`
	want := expectedFingerprintFloat(t, []float64{0.1, 0.2, 0.3})

	got, err := computeEmbeddingFingerprint([]byte(resp))
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestComputeEmbeddingFingerprint_PositiveZeroVsNegativeZero(t *testing.T) {
	posZero := `{"data":[{"embedding":[0.0]}]}`
	negZero := `{"data":[{"embedding":[-0.0]}]}`

	posFp, err := computeEmbeddingFingerprint([]byte(posZero))
	require.NoError(t, err)
	negFp, err := computeEmbeddingFingerprint([]byte(negZero))
	require.NoError(t, err)

	require.NotEqual(t, posFp, negFp, "+0 与 -0 位模式不同，指纹应不同")

	// 与独立复算对拍
	require.Equal(t, expectedFingerprintFloat(t, []float64{0.0}), posFp)
	require.Equal(t, expectedFingerprintFloat(t, []float64{math.Copysign(0, -1)}), negFp)
}

func TestComputeEmbeddingFingerprint_MultipleDataItems_Order(t *testing.T) {
	respAB := `{"data":[{"embedding":[0.1]},{"embedding":[0.2]}]}`
	respBA := `{"data":[{"embedding":[0.2]},{"embedding":[0.1]}]}`

	fpAB, err := computeEmbeddingFingerprint([]byte(respAB))
	require.NoError(t, err)
	fpBA, err := computeEmbeddingFingerprint([]byte(respBA))
	require.NoError(t, err)

	require.NotEqual(t, fpAB, fpBA, "不同顺序的 data 项应产生不同指纹")
	require.Equal(t, expectedFingerprintFloat(t, []float64{0.1}, []float64{0.2}), fpAB)
	require.Equal(t, expectedFingerprintFloat(t, []float64{0.2}, []float64{0.1}), fpBA)
}

func TestComputeEmbeddingFingerprint_Base64Strings(t *testing.T) {
	resp := `{"data":[{"embedding":"YWJjZA=="},{"embedding":"ZXZl"}]}`
	want := expectedFingerprintStrings(t, "YWJjZA==", "ZXZl")

	got, err := computeEmbeddingFingerprint([]byte(resp))
	require.NoError(t, err)
	require.Equal(t, want, got)
}

// TestComputeEmbeddingFingerprint_Base64Separator 消除拼接歧义：
// "ab"+"c" 与 "a"+"bc" 必须产生不同指纹（验证 0x00 分隔符生效）。
func TestComputeEmbeddingFingerprint_Base64Separator(t *testing.T) {
	respAB_C := `{"data":[{"embedding":"ab"},{"embedding":"c"}]}`
	respA_BC := `{"data":[{"embedding":"a"},{"embedding":"bc"}]}`

	fp1, err := computeEmbeddingFingerprint([]byte(respAB_C))
	require.NoError(t, err)
	fp2, err := computeEmbeddingFingerprint([]byte(respA_BC))
	require.NoError(t, err)

	require.NotEqual(t, fp1, fp2, "0x00 分隔符应消除拼接歧义，二者指纹必须不同")
	require.Equal(t, expectedFingerprintStrings(t, "ab", "c"), fp1)
	require.Equal(t, expectedFingerprintStrings(t, "a", "bc"), fp2)
}

func TestComputeEmbeddingFingerprint_InvalidJSON(t *testing.T) {
	_, err := computeEmbeddingFingerprint([]byte(`not a json`))
	require.Error(t, err)
}

func TestComputeEmbeddingFingerprint_EmptyData(t *testing.T) {
	_, err := computeEmbeddingFingerprint([]byte(`{"data":[]}`))
	require.Error(t, err)
}

func TestComputeEmbeddingFingerprint_MixedTypes(t *testing.T) {
	// 同一 data 中既有数组又有字符串 → 应返回 error
	resp := `{"data":[{"embedding":[0.1,0.2]},{"embedding":"YWJj"}]}`
	_, err := computeEmbeddingFingerprint([]byte(resp))
	require.Error(t, err)
}

// 额外保险：float 路径与字符串路径对同一向量值不应产生相同指纹（避免意外撞码）。
func TestComputeEmbeddingFingerprint_FloatVsStringPathsDiffer(t *testing.T) {
	floatResp := `{"data":[{"embedding":[0.1,0.2]}]}`
	strResp := `{"data":[{"embedding":"0.1,0.2"}]}`

	floatFp, err := computeEmbeddingFingerprint([]byte(floatResp))
	require.NoError(t, err)
	strFp, err := computeEmbeddingFingerprint([]byte(strResp))
	require.NoError(t, err)
	require.NotEqual(t, floatFp, strFp)
}

// 保险：确保 json 解析按预期工作（验证测试基础设施）。
func TestComputeEmbeddingFingerprint_JSONUnmarshalSanity(t *testing.T) {
	var r struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"data":[{"embedding":[0.1,0.2]}]}`), &r))
	require.Len(t, r.Data, 1)
	require.Equal(t, []float64{0.1, 0.2}, r.Data[0].Embedding)
}

// TestBuildTestRequestWebTools 测试网页工具端点的测试请求构建
func TestBuildTestRequestWebTools(t *testing.T) {
	searchReq, ok := buildTestRequest("deepseek-search", string(constant.EndpointTypeWebSearch), nil, false).(*dto.WebSearchRequest)
	require.True(t, ok)
	require.Equal(t, "deepseek-search", searchReq.Model)
	require.NotEmpty(t, searchReq.Query)
	require.NotNil(t, searchReq.Count)

	fetchReq, ok := buildTestRequest("deepseek-fetch", string(constant.EndpointTypeWebFetch), nil, false).(*dto.WebFetchRequest)
	require.True(t, ok)
	require.Equal(t, "deepseek-fetch", fetchReq.Model)
	require.NotEmpty(t, fetchReq.Url)
}

// TestNormalizeChannelTestEndpointWebTools 测试 DeepSeek 渠道 *-search/*-fetch 模型的端点自动识别
func TestNormalizeChannelTestEndpointWebTools(t *testing.T) {
	dsChannel := &model.Channel{Type: constant.ChannelTypeDeepSeek}
	require.Equal(t, string(constant.EndpointTypeWebSearch), normalizeChannelTestEndpoint(dsChannel, "deepseek-search", ""))
	require.Equal(t, string(constant.EndpointTypeWebFetch), normalizeChannelTestEndpoint(dsChannel, "deepseek-fetch", ""))
	// 显式指定端点时优先
	require.Equal(t, string(constant.EndpointTypeOpenAI), normalizeChannelTestEndpoint(dsChannel, "deepseek-search", string(constant.EndpointTypeOpenAI)))
	// 非 DeepSeek 渠道不做后缀自动识别，避免误伤 gpt-4o-search-preview 之类模型
	openaiChannel := &model.Channel{Type: constant.ChannelTypeOpenAI}
	require.Equal(t, "", normalizeChannelTestEndpoint(openaiChannel, "gpt-4o-search-preview", ""))
}

// TestNormalizeChannelTestEndpointAudio 测试 DeepSeek 渠道 *-tts/*-asr 虚拟模型的端点自动识别
func TestNormalizeChannelTestEndpointAudio(t *testing.T) {
	dsChannel := &model.Channel{Type: constant.ChannelTypeDeepSeek}
	require.Equal(t, string(constant.EndpointTypeTTS), normalizeChannelTestEndpoint(dsChannel, "deepseek-tts", ""))
	require.Equal(t, string(constant.EndpointTypeASR), normalizeChannelTestEndpoint(dsChannel, "deepseek-asr", ""))
	// 显式指定端点时优先
	require.Equal(t, string(constant.EndpointTypeOpenAI), normalizeChannelTestEndpoint(dsChannel, "deepseek-tts", string(constant.EndpointTypeOpenAI)))
	// 非 DeepSeek 渠道不做后缀自动识别
	openaiChannel := &model.Channel{Type: constant.ChannelTypeOpenAI}
	require.Equal(t, "", normalizeChannelTestEndpoint(openaiChannel, "gpt-4o-tts", ""))
}

// TestBuildTestRequestAudio 测试语音端点的测试请求构建
func TestBuildTestRequestAudio(t *testing.T) {
	ttsReq, ok := buildTestRequest("deepseek-tts", string(constant.EndpointTypeTTS), nil, false).(*dto.AudioRequest)
	require.True(t, ok)
	require.Equal(t, "deepseek-tts", ttsReq.Model)
	require.Equal(t, "wav", ttsReq.ResponseFormat)
	require.NotEmpty(t, ttsReq.Input)

	asrReq, ok := buildTestRequest("deepseek-asr", string(constant.EndpointTypeASR), nil, false).(*dto.AudioRequest)
	require.True(t, ok)
	require.Equal(t, "deepseek-asr", asrReq.Model)
	require.Equal(t, "json", asrReq.ResponseFormat)
}

// TestBuildTestAudioTranscriptionBody 测试语音识别渠道测试用的 multipart 请求体
func TestBuildTestAudioTranscriptionBody(t *testing.T) {
	body, contentType, err := buildTestAudioTranscriptionBody("deepseek-asr")
	require.NoError(t, err)
	require.Contains(t, contentType, "multipart/form-data")

	_, params, err := mime.ParseMediaType(contentType)
	require.NoError(t, err)

	form, err := multipart.NewReader(bytes.NewReader(body), params["boundary"]).ReadForm(1 << 20)
	require.NoError(t, err)
	defer form.RemoveAll()

	require.Equal(t, []string{"deepseek-asr"}, form.Value["model"])
	require.Len(t, form.File["file"], 1)
	require.Equal(t, "channel_test.wav", form.File["file"][0].Filename)

	file, err := form.File["file"][0].Open()
	require.NoError(t, err)
	defer file.Close()
	header := make([]byte, 12)
	_, err = file.Read(header)
	require.NoError(t, err)
	require.Equal(t, "RIFF", string(header[0:4]))
	require.Equal(t, "WAVE", string(header[8:12]))

	// 生成的 WAV 应可被时长解析器识别为 1 秒，保证上游转写请求格式有效
	duration, err := common.GetAudioDuration(context.Background(), bytes.NewReader(buildTestWAV()), ".wav")
	require.NoError(t, err)
	require.InDelta(t, 1.0, duration, 0.001)
}
