package doubao

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// TestConvertToRequestPayloadImplicitImagesUnchanged 锚定现状（零回归）：
// 隐式路径（images 数组）保持无 role 的 image_url 项 ——
// 上游 role 合法取值域待核实，不做推测。
func TestConvertToRequestPayloadImplicitImagesUnchanged(t *testing.T) {
	a := &TaskAdaptor{}
	payload, err := a.convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Prompt: "p",
		Model:  "doubao-seedance-1-0",
		Images: []string{"https://x/a.png", "https://x/b.png"},
	})
	if err != nil {
		t.Fatalf("convertToRequestPayload() error = %v", err)
	}

	var imageItems []ContentItem
	for _, item := range payload.Content {
		if item.Type == "image_url" {
			imageItems = append(imageItems, item)
		}
	}
	if len(imageItems) != 2 {
		t.Fatalf("image items = %d, want 2", len(imageItems))
	}
	for i, item := range imageItems {
		if item.Role != "" {
			t.Errorf("imageItems[%d].Role = %q, want empty (implicit path keeps no role)", i, item.Role)
		}
	}
}

// TestConvertToRequestPayloadExplicitRoles 阶段 1 增量（D6 修复）：
// 显式路径（metadata 具名键）为 content[].role 赋值。
func TestConvertToRequestPayloadExplicitRoles(t *testing.T) {
	a := &TaskAdaptor{}
	payload, err := a.convertToRequestPayload(&relaycommon.TaskSubmitReq{
		Prompt: "p",
		Model:  "doubao-seedance-1-0",
		Metadata: map[string]interface{}{
			relaycommon.MetadataKeyFirstFrame:      "https://x/first.png",
			relaycommon.MetadataKeyLastFrame:       "https://x/last.png",
			relaycommon.MetadataKeyReferenceImages: []string{"https://x/r1.png"},
			relaycommon.MetadataKeyReferenceVideos: []string{"https://x/v1.mp4"},
			relaycommon.MetadataKeyReferenceAudios: []string{"https://x/a1.mp3"},
		},
	})
	if err != nil {
		t.Fatalf("convertToRequestPayload() error = %v", err)
	}

	var got []ContentItem
	for _, item := range payload.Content {
		if item.Type != "text" {
			got = append(got, item)
		}
	}
	if len(got) != 5 {
		t.Fatalf("media items = %d, want 5 (got %+v)", len(got), got)
	}

	expect := []struct {
		itemType string
		role     string
		url      string
	}{
		{"image_url", doubaoRoleFirstFrame, "https://x/first.png"},
		{"image_url", doubaoRoleLastFrame, "https://x/last.png"},
		{"image_url", doubaoRoleReferenceImage, "https://x/r1.png"},
		{"video_url", doubaoRoleReferenceVideo, "https://x/v1.mp4"},
		{"audio_url", doubaoRoleReferenceAudio, "https://x/a1.mp3"},
	}
	for i, e := range expect {
		if got[i].Type != e.itemType || got[i].Role != e.role {
			t.Errorf("item[%d] = (type=%q, role=%q), want (type=%q, role=%q)",
				i, got[i].Type, got[i].Role, e.itemType, e.role)
		}
		var url string
		switch {
		case got[i].ImageURL != nil:
			url = got[i].ImageURL.URL
		case got[i].VideoURL != nil:
			url = got[i].VideoURL.URL
		case got[i].AudioURL != nil:
			url = got[i].AudioURL.URL
		}
		if url != e.url {
			t.Errorf("item[%d] url = %q, want %q", i, url, e.url)
		}
	}

	// prompt 仍作为 text 项追加在末尾（现状行为不变）
	last := payload.Content[len(payload.Content)-1]
	if last.Type != "text" || last.Text != "p" {
		t.Errorf("last item = %+v, want text prompt", last)
	}
}
