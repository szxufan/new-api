package constant

type TaskPlatform string

const (
	TaskPlatformSuno       TaskPlatform = "suno"
	TaskPlatformMidjourney              = "mj"
)

const (
	SunoActionMusic  = "MUSIC"
	SunoActionLyrics = "LYRICS"

	TaskActionGenerate          = "generate"
	TaskActionTextGenerate      = "textGenerate"
	TaskActionFirstTailGenerate = "firstTailGenerate"
	TaskActionReferenceGenerate = "referenceGenerate"
	TaskActionRemix             = "remixGenerate"
	// TaskActionVideoV2Generate 标记 MiniMax 视频 v2（H3）协议任务，随 task.Action 持久化，
	// 供后台轮询侧选择 v2 查询端点（轮询仅能拿到 task_id 与 action，拿不到模型名）。
	TaskActionVideoV2Generate = "videoV2Generate"
)

var SunoModel2Action = map[string]string{
	"suno_music":  SunoActionMusic,
	"suno_lyrics": SunoActionLyrics,
}
