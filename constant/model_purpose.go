package constant

const (
	ModelPurposeChat            = "chat"
	ModelPurposeProgramming     = "programming"
	ModelPurposeWriting         = "writing"
	ModelPurposeASR             = "asr"
	ModelPurposeTTS             = "tts"
	ModelPurposeImageGeneration = "image_generation"
	ModelPurposeImageEdit       = "image_edit"
	ModelPurposeVideoGeneration = "video_generation"
	ModelPurposeRealtime        = "realtime"
	ModelPurposeTranslation     = "translation"
)

// ModelPurposeList 是模型用途的固定预设及展示顺序。
var ModelPurposeList = []string{
	ModelPurposeChat,
	ModelPurposeProgramming,
	ModelPurposeWriting,
	ModelPurposeASR,
	ModelPurposeTTS,
	ModelPurposeImageGeneration,
	ModelPurposeImageEdit,
	ModelPurposeVideoGeneration,
	ModelPurposeRealtime,
	ModelPurposeTranslation,
}

func IsValidModelPurpose(purpose string) bool {
	for _, validPurpose := range ModelPurposeList {
		if purpose == validPurpose {
			return true
		}
	}
	return false
}
