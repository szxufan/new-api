package ali

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

// qwenImageMaxInputImages 千问图像系列图生图（I2I）场景最多支持的输入图像数量。
const qwenImageMaxInputImages = 3

func oaiImage2AliImageRequest(info *relaycommon.RelayInfo, request dto.ImageRequest, isSync bool) (*AliImageRequest, error) {
	var imageRequest AliImageRequest
	imageRequest.Model = request.Model
	if request.Extra != nil {
		if val, ok := request.Extra["parameters"]; ok {
			err := common.Unmarshal(val, &imageRequest.Parameters)
			if err != nil {
				return nil, fmt.Errorf("invalid parameters field: %w", err)
			}
		} else {
			// 兼容没有parameters字段的情况，从openai标准字段中提取参数
			imageRequest.Parameters = AliImageParameters{
				Size:      strings.Replace(request.Size, "x", "*", -1),
				N:         int(lo.FromPtrOr(request.N, uint(1))),
				Watermark: request.Watermark,
			}
		}
		if val, ok := request.Extra["input"]; ok {
			err := common.Unmarshal(val, &imageRequest.Input)
			if err != nil {
				return nil, fmt.Errorf("invalid input field: %w", err)
			}
		}
	}

	if strings.Contains(request.Model, "z-image") {
		// z-image 开启prompt_extend后，按2倍计费
		if imageRequest.Parameters.PromptExtendValue() {
			info.PriceData.AddOtherRatio("prompt_extend", 2)
		}
	}

	if imageRequest.Parameters.N != 0 {
		info.PriceData.AddOtherRatio("n", float64(imageRequest.Parameters.N))
	}

	// 同步图片模型和异步图片模型请求格式不一样
	if isSync {
		if imageRequest.Input == nil {
			images, err := getImageInputsFromJSON(request)
			if err != nil {
				return nil, err
			}
			content, err := buildAliSyncContent(request.Model, request.Prompt, images)
			if err != nil {
				return nil, err
			}
			imageRequest.Input = AliImageInput{
				Messages: []AliMessage{
					{
						Role:    "user",
						Content: content,
					},
				},
			}
		}
	} else {
		if imageRequest.Input == nil {
			imageRequest.Input = AliImageInput{
				Prompt: request.Prompt,
			}
		}
	}

	return &imageRequest, nil
}

// buildAliSyncContent 构造同步图像模型 input.messages[].content 数组。
// 图生图（I2I）场景下按顺序放入全部输入图像，最后追加唯一的文本指令；
// 文生图（T2I）场景下仅包含文本指令。
// 参考: https://help.aliyun.com/zh/model-studio/qwen-image-generation-and-editing-api-reference
func buildAliSyncContent(model string, prompt string, images []string) ([]AliMediaContent, error) {
	if isQwenImageModel(model) && len(images) > qwenImageMaxInputImages {
		return nil, fmt.Errorf("qwen-image supports at most %d input images, got %d", qwenImageMaxInputImages, len(images))
	}

	content := make([]AliMediaContent, 0, len(images)+1)
	for _, image := range images {
		content = append(content, AliMediaContent{Image: image})
	}
	content = append(content, AliMediaContent{Text: prompt})
	return content, nil
}

// isQwenImageModel 判断是否为千问图像系列模型（qwen-image / qwen-image-edit / qwen-image-3.0 等）。
func isQwenImageModel(modelName string) bool {
	return strings.Contains(strings.ToLower(modelName), "qwen-image")
}

// getImageInputsFromJSON 从 JSON 请求体中提取输入图像，兼容以下写法：
//   - "image": "https://..." 或 "data:image/png;base64,..."
//   - "image": ["...", "..."]
//   - "images": ["...", "..."]
func getImageInputsFromJSON(request dto.ImageRequest) ([]string, error) {
	raw := request.Image
	if len(raw) == 0 {
		raw = request.Images
	}
	if len(raw) == 0 {
		return nil, nil
	}

	var single string
	if err := common.Unmarshal(raw, &single); err == nil {
		return normalizeImageInputs([]string{single})
	}

	var list []string
	if err := common.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("invalid image field: %w", err)
	}
	return normalizeImageInputs(list)
}

// normalizeImageInputs 丢弃空值，并把裸 base64 补全为 DashScope 要求的 data:{MIME_type};base64,{data} 格式。
func normalizeImageInputs(inputs []string) ([]string, error) {
	images := make([]string, 0, len(inputs))
	for _, input := range inputs {
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		if strings.HasPrefix(input, "data:") || strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
			images = append(images, input)
			continue
		}
		mimeType, base64Data, err := service.DecodeBase64FileData(input)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 image: %w", err)
		}
		images = append(images, fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data))
	}
	if len(images) == 0 {
		return nil, nil
	}
	return images, nil
}
func getImageBase64sFromForm(c *gin.Context, fieldName string) ([]string, error) {
	mf := c.Request.MultipartForm
	if mf == nil {
		if _, err := c.MultipartForm(); err != nil {
			return nil, fmt.Errorf("failed to parse image edit form request: %w", err)
		}
		mf = c.Request.MultipartForm
	}

	var imageFiles []*multipart.FileHeader
	var exists bool

	// First check for standard "image" field
	if imageFiles, exists = mf.File["image"]; !exists || len(imageFiles) == 0 {
		// If not found, check for "image[]" field
		if imageFiles, exists = mf.File["image[]"]; !exists || len(imageFiles) == 0 {
			// If still not found, iterate through all fields to find any that start with "image["
			foundArrayImages := false
			for fieldName, files := range mf.File {
				if strings.HasPrefix(fieldName, "image[") && len(files) > 0 {
					foundArrayImages = true
					imageFiles = append(imageFiles, files...)
				}
			}

			// If no image fields found at all
			if !foundArrayImages && (len(imageFiles) == 0) {
				return nil, errors.New("image is required")
			}
		}
	}

	if len(imageFiles) == 0 {
		return nil, errors.New("image is required")
	}

	// 获取base64编码的图片
	var imageBase64s []string
	for _, file := range imageFiles {
		image, err := file.Open()
		if err != nil {
			return nil, errors.New("failed to open image file")
		}

		// 读取文件内容
		imageData, err := io.ReadAll(image)
		if err != nil {
			return nil, errors.New("failed to read image file")
		}

		// 获取MIME类型
		mimeType := http.DetectContentType(imageData)

		// 编码为base64
		base64Data := base64.StdEncoding.EncodeToString(imageData)

		// 构造data URL格式
		dataURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)
		imageBase64s = append(imageBase64s, dataURL)
		image.Close()
	}
	return imageBase64s, nil
}

func oaiFormEdit2AliImageEdit(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (*AliImageRequest, error) {
	var imageRequest AliImageRequest
	imageRequest.Model = request.Model

	imageBase64s, err := getImageBase64sFromForm(c, "image")
	if err != nil {
		return nil, fmt.Errorf("get image base64s from form failed: %w", err)
	}
	content, err := buildAliSyncContent(request.Model, request.Prompt, imageBase64s)
	if err != nil {
		return nil, err
	}
	imageRequest.Input = AliImageInput{
		Messages: []AliMessage{
			{
				Role:    "user",
				Content: content,
			},
		},
	}
	imageRequest.Parameters = AliImageParameters{
		Size:      strings.Replace(request.Size, "x", "*", -1),
		N:         int(lo.FromPtrOr(request.N, uint(1))),
		Watermark: request.Watermark,
	}
	return &imageRequest, nil
}

func updateTask(info *relaycommon.RelayInfo, taskID string) (*AliResponse, error, []byte) {
	url := fmt.Sprintf("%s/api/v1/tasks/%s", info.ChannelBaseUrl, taskID)

	var aliResponse AliResponse

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &aliResponse, err, nil
	}

	req.Header.Set("Authorization", "Bearer "+info.ApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		common.SysLog("updateTask client.Do err: " + err.Error())
		return &aliResponse, err, nil
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)

	var response AliResponse
	err = common.Unmarshal(responseBody, &response)
	if err != nil {
		common.SysLog("updateTask NewDecoder err: " + err.Error())
		return &aliResponse, err, nil
	}

	return &response, nil, responseBody
}

func asyncTaskWait(c *gin.Context, info *relaycommon.RelayInfo, taskID string) (*AliResponse, []byte, error) {
	waitSeconds := 10
	step := 0
	maxStep := 20

	var taskResponse AliResponse
	var responseBody []byte

	time.Sleep(time.Duration(5) * time.Second)

	for {
		logger.LogDebug(c, "asyncTaskWait step %d/%d, wait %d seconds", step, maxStep, waitSeconds)
		step++
		rsp, err, body := updateTask(info, taskID)
		responseBody = body
		if err != nil {
			logger.LogWarn(c, "asyncTaskWait UpdateTask err: "+err.Error())
			time.Sleep(time.Duration(waitSeconds) * time.Second)
			continue
		}

		if rsp.Output.TaskStatus == "" {
			return &taskResponse, responseBody, nil
		}

		switch rsp.Output.TaskStatus {
		case "FAILED":
			fallthrough
		case "CANCELED":
			fallthrough
		case "SUCCEEDED":
			fallthrough
		case "UNKNOWN":
			return rsp, responseBody, nil
		}
		if step >= maxStep {
			break
		}
		time.Sleep(time.Duration(waitSeconds) * time.Second)
	}

	return nil, nil, fmt.Errorf("aliAsyncTaskWait timeout")
}

func responseAli2OpenAIImage(c *gin.Context, response *AliResponse, originBody []byte, info *relaycommon.RelayInfo, responseFormat string) *dto.ImageResponse {
	imageResponse := dto.ImageResponse{
		Created: info.StartTime.Unix(),
	}

	if len(response.Output.Results) > 0 {
		imageResponse.Data = response.Output.ResultToOpenAIImageDate(c, responseFormat)
	} else if len(response.Output.Choices) > 0 {
		imageResponse.Data = response.Output.ChoicesToOpenAIImageDate(c, responseFormat)
	}

	imageResponse.Metadata = originBody
	return &imageResponse
}

func aliImageHandler(a *Adaptor, c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*types.NewAPIError, *dto.Usage) {
	responseFormat := c.GetString("response_format")

	var aliTaskResponse AliResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError), nil
	}
	service.CloseResponseBodyGracefully(resp)
	err = common.Unmarshal(responseBody, &aliTaskResponse)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError), nil
	}

	if aliTaskResponse.Message != "" {
		logger.LogError(c, "ali_async_task_failed: "+aliTaskResponse.Message)
		return types.NewError(errors.New(aliTaskResponse.Message), types.ErrorCodeBadResponse), nil
	}

	var (
		aliResponse    *AliResponse
		originRespBody []byte
	)

	if a.IsSyncImageModel {
		aliResponse = &aliTaskResponse
		originRespBody = responseBody
	} else {
		// 异步图片模型需要轮询任务结果
		aliResponse, originRespBody, err = asyncTaskWait(c, info, aliTaskResponse.Output.TaskId)
		if err != nil {
			return types.NewError(err, types.ErrorCodeBadResponse), nil
		}
		if aliResponse.Output.TaskStatus != "SUCCEEDED" {
			return types.WithOpenAIError(types.OpenAIError{
				Message: aliResponse.Output.Message,
				Type:    "ali_error",
				Param:   "",
				Code:    aliResponse.Output.Code,
			}, resp.StatusCode), nil
		}
	}

	if a.IsSyncImageModel {
		logger.LogDebug(c, "ali_sync_image_result: %s", originRespBody)
	} else {
		logger.LogDebug(c, "ali_async_image_result: %s", originRespBody)
	}

	imageResponses := responseAli2OpenAIImage(c, aliResponse, originRespBody, info, responseFormat)
	if aliResponse.Usage.ImageCount != 0 {
		info.PriceData.AddOtherRatio("n", float64(aliResponse.Usage.ImageCount))
	} else if len(imageResponses.Data) != 0 {
		info.PriceData.AddOtherRatio("n", float64(len(imageResponses.Data)))
	}
	jsonResponse, err := common.Marshal(imageResponses)
	if err != nil {
		return types.NewError(err, types.ErrorCodeBadResponseBody), nil
	}
	service.IOCopyBytesGracefully(c, resp, jsonResponse)

	return nil, &dto.Usage{}
}
