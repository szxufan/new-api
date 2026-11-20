package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func withSelfUseModeEnabled(t *testing.T) {
	t.Helper()
	original := operation_setting.SelfUseModeEnabled
	operation_setting.SelfUseModeEnabled = true
	t.Cleanup(func() {
		operation_setting.SelfUseModeEnabled = original
	})
}

func TestOpenAIModelsReturnNativePurposes(t *testing.T) {
	withSelfUseModeEnabled(t)
	db := setupModelListControllerTestDB(t)

	require.NoError(t, db.Create(&model.User{
		Id:       1101,
		Username: "model-purpose-user",
		Password: "password",
		Group:    "default",
		Status:   common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Channel{
		Id:     1,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "test-key",
		Status: common.ChannelStatusEnabled,
		Name:   "model-purpose-channel",
	}).Error)
	require.NoError(t, db.Create(&[]model.Ability{
		{Group: "default", Model: "zz-purpose-exact", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-purpose-prefixed", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-purpose-empty", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-purpose-unset", ChannelId: 1, Enabled: true},
		{Group: "default", Model: "zz-purpose-corrupt", ChannelId: 1, Enabled: true},
	}).Error)
	require.NoError(t, db.Create(&[]model.Model{
		{ModelName: "zz-purpose-exact", Status: 1, Purposes: `["chat","chat","programming"]`},
		{ModelName: "zz-purpose-", Status: 1, NameRule: model.NameRulePrefix, Purposes: `["writing"]`},
		{ModelName: "zz-purpose-empty", Status: 1, Purposes: `[]`},
		{ModelName: "zz-purpose-unset", Status: 1},
		{ModelName: "zz-purpose-corrupt", Status: 1, Purposes: `["coding"]`},
		{ModelName: "gpt-4o", Status: 1, Purposes: `["programming","chat"]`},
	}).Error)
	model.RefreshPricing()

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	ctx.Set("id", 1101)
	ListModels(ctx, constant.ChannelTypeOpenAI)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload listModelsResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	items := make(map[string]dto.OpenAIModels, len(payload.Data))
	for _, item := range payload.Data {
		items[item.Id] = item
	}
	require.Equal(t, []string{"chat", "programming"}, items["zz-purpose-exact"].Purposes)
	require.Equal(t, []string{"writing"}, items["zz-purpose-prefixed"].Purposes)
	require.Empty(t, items["zz-purpose-empty"].Purposes)
	require.Empty(t, items["zz-purpose-unset"].Purposes)
	require.Empty(t, items["zz-purpose-corrupt"].Purposes)

	var rawPayload struct {
		Data []map[string]json.RawMessage `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &rawPayload))
	rawItems := make(map[string]map[string]json.RawMessage, len(rawPayload.Data))
	for _, item := range rawPayload.Data {
		var id string
		require.NoError(t, common.Unmarshal(item["id"], &id))
		rawItems[id] = item
	}

	rawPurposes, ok := rawItems["zz-purpose-exact"]["purposes"]
	require.True(t, ok)
	require.Equal(t, json.RawMessage(`["chat","programming"]`), rawPurposes)
	var nativePurposes []string
	require.NoError(t, common.Unmarshal(rawPurposes, &nativePurposes))
	require.Equal(t, []string{"chat", "programming"}, nativePurposes)
	for _, modelName := range []string{"zz-purpose-empty", "zz-purpose-unset", "zz-purpose-corrupt"} {
		_, exists := rawItems[modelName]["purposes"]
		require.False(t, exists, "purposes should be omitted for %s", modelName)
	}

	require.Contains(t, openAIModelsMap, "gpt-4o")
	retrieveRecorder := httptest.NewRecorder()
	retrieveCtx, _ := gin.CreateTestContext(retrieveRecorder)
	retrieveCtx.Request = httptest.NewRequest(http.MethodGet, "/v1/models/gpt-4o", nil)
	retrieveCtx.Params = []gin.Param{{Key: "model", Value: "gpt-4o"}}
	RetrieveModel(retrieveCtx, constant.ChannelTypeOpenAI)

	require.Equal(t, http.StatusOK, retrieveRecorder.Code)
	var retrieved dto.OpenAIModels
	require.NoError(t, common.Unmarshal(retrieveRecorder.Body.Bytes(), &retrieved))
	require.Equal(t, []string{"programming", "chat"}, retrieved.Purposes)
}

type modelPurposeAPIResponse struct {
	Success bool         `json:"success"`
	Message string       `json:"message"`
	Data    *model.Model `json:"data"`
}

func performModelPurposeRequest(t *testing.T, handler func(*gin.Context), method string, target string, payload map[string]any) (*httptest.ResponseRecorder, modelPurposeAPIResponse) {
	t.Helper()
	body, err := common.Marshal(payload)
	require.NoError(t, err)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(method, target, bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	handler(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response modelPurposeAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	return recorder, response
}

func TestModelMetaPurposesValidationAndClearing(t *testing.T) {
	db := setupModelListControllerTestDB(t)

	_, response := performModelPurposeRequest(t, CreateModelMeta, http.MethodPost, "/api/models/", map[string]any{
		"model_name": "managed-purpose-invalid",
		"status":     1,
		"purposes":   `["chat","coding"]`,
	})
	require.False(t, response.Success)
	require.Contains(t, response.Message, "coding")
	var count int64
	require.NoError(t, db.Model(&model.Model{}).Where("model_name = ?", "managed-purpose-invalid").Count(&count).Error)
	require.Zero(t, count)

	_, response = performModelPurposeRequest(t, CreateModelMeta, http.MethodPost, "/api/models/", map[string]any{
		"model_name": "managed-purpose",
		"status":     1,
		"purposes":   `["chat","chat","writing"]`,
	})
	require.True(t, response.Success, response.Message)
	require.NotNil(t, response.Data)
	modelID := response.Data.Id
	require.Equal(t, `["chat","writing"]`, response.Data.Purposes)

	var stored model.Model
	require.NoError(t, db.First(&stored, modelID).Error)
	require.Equal(t, `["chat","writing"]`, stored.Purposes)

	_, response = performModelPurposeRequest(t, UpdateModelMeta, http.MethodPut, "/api/models/", map[string]any{
		"id":         modelID,
		"model_name": "managed-purpose",
		"status":     1,
		"purposes":   `["programming","unknown"]`,
	})
	require.False(t, response.Success)
	require.Contains(t, response.Message, "unknown")
	require.NoError(t, db.First(&stored, modelID).Error)
	require.Equal(t, `["chat","writing"]`, stored.Purposes)

	_, response = performModelPurposeRequest(t, UpdateModelMeta, http.MethodPut, "/api/models/?status_only=true", map[string]any{
		"id":       modelID,
		"status":   0,
		"purposes": `["unknown"]`,
	})
	require.True(t, response.Success, response.Message)
	require.NoError(t, db.First(&stored, modelID).Error)
	require.Equal(t, 0, stored.Status)
	require.Equal(t, `["chat","writing"]`, stored.Purposes)

	_, response = performModelPurposeRequest(t, UpdateModelMeta, http.MethodPut, "/api/models/", map[string]any{
		"id":         modelID,
		"model_name": "managed-purpose",
		"status":     1,
		"purposes":   "",
	})
	require.True(t, response.Success, response.Message)
	require.NotNil(t, response.Data)
	require.Empty(t, response.Data.Purposes)
	require.NoError(t, db.First(&stored, modelID).Error)
	require.Empty(t, stored.Purposes)
}
