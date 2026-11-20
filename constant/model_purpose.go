package constant

const (
	ModelPurposeChat        = "chat"
	ModelPurposeProgramming = "programming"
	ModelPurposeWriting     = "writing"
)

// ModelPurposeList 是模型用途的固定预设及展示顺序。
var ModelPurposeList = []string{
	ModelPurposeChat,
	ModelPurposeProgramming,
	ModelPurposeWriting,
}

func IsValidModelPurpose(purpose string) bool {
	for _, validPurpose := range ModelPurposeList {
		if purpose == validPurpose {
			return true
		}
	}
	return false
}
