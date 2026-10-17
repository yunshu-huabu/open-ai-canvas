package protocol

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func vintedTestRequest() GenerationRequest {
	return GenerationRequest{Model: "seedance2.0fast", Prompt: "参考@图片1和@视频1，配合@音频1生成视频", Extra: map[string]any{"idempotencyKey": "test-operation"}}
}

func TestVintedAccountPriceIncludesOnlyApplicableNativeFaceFee(t *testing.T) {
	adapter := officialPackageAdapter(t, "vinted-seedance-video.yingce-plugin", "vinted-seedance-video")
	var pricing ManifestPreparation
	for _, step := range adapter.Metadata().Lifecycle.Prepare {
		if step.ID == "pricing" {
			pricing = step
		}
	}
	for _, tc := range []struct {
		name   string
		visual bool
		face   any
		want   float64
	}{
		{"text_default", false, nil, 10}, {"reference_default", true, nil, 15}, {"reference_off", true, false, 10}, {"reference_on", true, true, 15},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := vintedTestRequest()
			if tc.visual {
				request.Images = []MediaReference{{URL: "https://cdn.example/image.png"}}
			}
			if tc.face != nil {
				request.ProviderOptions = map[string]map[string]any{"vinted-seedance-video": {"native_face": tc.face}}
			}
			value, _, err := adapter.(PreparationAdapter).ParsePreparation(context.Background(), pricing, request, []byte(`{"items":[{"model":"seedance2.0fast","duration":5,"credits":10}],"native_face_surcharge_credits":5}`))
			if err != nil {
				t.Fatal(err)
			}
			if manifestFloat(value.(map[string]any)["credits"]) != tc.want {
				t.Fatalf("quote=%#v want=%v", value, tc.want)
			}
		})
	}
}

func TestVintedVideoRequestAndLifecycle(t *testing.T) {
	adapter := officialPackageAdapter(t, "vinted-seedance-video.yingce-plugin", "vinted-seedance-video")
	request := vintedTestRequest()
	request.Model, request.Duration, request.AspectRatio = "seedance2.5", 23, "9:16"
	request.Images = []MediaReference{{URL: "https://cdn.example/second.png", Order: 2}, {URL: "https://cdn.example/first.png", Order: 1}}
	request.Videos = []MediaReference{{URL: "https://cdn.example/motion.mp4"}}
	request.Audios = []MediaReference{{URL: "https://cdn.example/voice.mp3"}}
	request.ProviderOptions = map[string]map[string]any{"vinted-seedance-video": {"native_face": false, "camera_movement": "fixed", "image_ids": []string{"uploaded-image"}}}
	request.Extra["preparation"] = map[string]any{"images": []any{"first-image", "second-image"}}
	request.Videos[0].DataURL = "data:video/mp4;base64,AAAAAGZ0eXBpc29t"
	request.Audios[0].DataURL = "data:audio/mpeg;base64,SUQzAAAAAAAAAAAA"
	spec, err := adapter.BuildCreate(context.Background(), RequestContext{Request: request})
	if err != nil {
		t.Fatal(err)
	}
	body := manifestTestBody(t, spec)
	if spec.Path != "/v1/videos" || spec.Method != "POST" || spec.Auth.Type != "bearer" || spec.Headers["Idempotency-Key"] != "test-operation" || adapter.Metadata().RequiresPublicMediaURLs || adapter.Metadata().Lifecycle == nil {
		t.Fatalf("spec = %#v", spec)
	}
	if body["duration"] != float64(23) || body["ratio"] != "9:16" || body["resolution"] != "720p" || body["native_face"] != false || body["camera_movement"] != "fixed" {
		t.Fatalf("body = %#v", body)
	}
	for _, field := range []string{"seconds", "mode", "input_references", "generate_audio", "watermark", "idempotencyKey"} {
		if _, exists := body[field]; exists {
			t.Fatalf("unexpected %s in body", field)
		}
	}
	for field, expected := range map[string][]string{"video_b64s": {request.Videos[0].DataURL}, "audio_b64s": {request.Audios[0].DataURL}, "image_ids": {"uploaded-image", "first-image", "second-image"}} {
		values, _ := body[field].([]any)
		if len(values) != len(expected) {
			t.Fatalf("%s = %#v", field, values)
		}
		for i, value := range expected {
			if values[i] != value {
				t.Fatalf("%s = %#v", field, values)
			}
		}
	}
	created, err := adapter.ParseCreate(context.Background(), []byte(`{"id":"job-1","status":"queued"}`))
	if err != nil || created.TaskID != "job-1" || created.Status != StatusPending {
		t.Fatalf("created = %#v, err = %v", created, err)
	}
	pollContext := PollContext{TaskID: "job-1", Request: request}
	poll, err := adapter.BuildPoll(context.Background(), pollContext)
	if err != nil || poll.Path != "/v1/videos/job-1" {
		t.Fatalf("poll = %#v, err = %v", poll, err)
	}
	for _, tc := range []struct {
		body   string
		status Status
	}{
		{`{"status":"running","download_url":"https://cdn.example/preview.mp4"}`, StatusProcessing},
		{`{"status":"succeeded","url":"https://cdn.example/preview.mp4"}`, StatusSucceeded},
		{`{"status":"failed","error":"moderation failed"}`, StatusFailed},
		{`{"quality":"preview","url":"https://cdn.example/preview.mp4"}`, StatusFailed},
	} {
		state, err := adapter.ParsePoll(context.Background(), pollContext, []byte(tc.body))
		if err != nil || state.Status != tc.status || state.Result != nil {
			t.Fatalf("state = %#v, err = %v", state, err)
		}
	}
	result, err := adapter.(ResultAdapter).BuildResult(context.Background(), pollContext)
	if err != nil || result.Path != "/v1/videos/job-1/signed_url" || result.Query["download"][0] != "1" {
		t.Fatalf("result = %#v, err = %v", result, err)
	}
	state, err := adapter.ParsePoll(context.Background(), pollContext, []byte(`{"quality":"original","url":"/v1/videos/job-1/file?signature=abc%2Bdef&version=original"}`))
	if err != nil || state.Result == nil || len(state.Result.Videos) != 1 || !state.Result.Videos[0].Ephemeral {
		t.Fatalf("signed result = %#v, err = %v", state, err)
	}
}

func TestVintedVideoModelDurations(t *testing.T) {
	adapter := officialPackageAdapter(t, "vinted-seedance-video.yingce-plugin", "vinted-seedance-video")
	for _, model := range []string{"seedance2.5", "seedance2.0", "seedance2.0fast", "seedance2.0mini"} {
		for duration := -1; duration <= 31; duration++ {
			t.Run(fmt.Sprintf("%s/%d", model, duration), func(t *testing.T) {
				request := vintedTestRequest()
				request.Model, request.Duration = model, duration
				valid := duration == 0 || (model == "seedance2.5" && duration >= 16 && duration <= 30) || (model != "seedance2.5" && (duration == 5 || duration == 10 || (model != "seedance2.0mini" && duration == 15)))
				spec, err := adapter.BuildCreate(context.Background(), RequestContext{Request: request})
				if (err == nil) != valid {
					t.Fatalf("valid = %v, err = %v", valid, err)
				}
				if duration == 0 && err == nil {
					want := float64(5)
					if model == "seedance2.5" {
						want = 30
					}
					body := manifestTestBody(t, spec)
					if body["duration"] != want {
						t.Fatalf("default duration = %v", body["duration"])
					}
					if _, exists := body["native_face"]; exists {
						t.Fatal("unset native_face must preserve upstream default")
					}
				}
			})
		}
	}
}

func TestVintedVideoRejectsUnsupportedInputs(t *testing.T) {
	adapter := officialPackageAdapter(t, "vinted-seedance-video.yingce-plugin", "vinted-seedance-video")
	for _, tc := range []struct {
		name   string
		change func(*GenerationRequest)
	}{
		{"missing_idempotency", func(r *GenerationRequest) { r.Extra = nil }},
		{"niulai_model", func(r *GenerationRequest) { r.Model = "sd-2-c4" }},
		{"empty_prompt", func(r *GenerationRequest) { r.Prompt = " " }},
		{"long_prompt", func(r *GenerationRequest) { r.Prompt = strings.Repeat("中", 15001) }},
		{"resolution", func(r *GenerationRequest) { r.Resolution = "1080p" }},
		{"ratio", func(r *GenerationRequest) { r.AspectRatio = "adaptive" }},
		{"image_limit", func(r *GenerationRequest) {
			for i := 0; i < 10; i++ {
				r.Images = append(r.Images, MediaReference{URL: "https://cdn.example/image.png"})
			}
		}},
		{"audio_limit", func(r *GenerationRequest) {
			for i := 0; i < 4; i++ {
				r.Audios = append(r.Audios, MediaReference{URL: "https://cdn.example/audio.mp3"})
			}
		}},
		{"video_limit", func(r *GenerationRequest) {
			for i := 0; i < 4; i++ {
				r.Videos = append(r.Videos, MediaReference{URL: "https://cdn.example/video.mp4"})
			}
		}},
		{"invalid_image_id", func(r *GenerationRequest) {
			r.ProviderOptions = map[string]map[string]any{"vinted-seedance-video": {"image_ids": []string{"invalid/id"}}}
		}},
		{"duplicate_image_id", func(r *GenerationRequest) {
			r.ProviderOptions = map[string]map[string]any{"vinted-seedance-video": {"image_ids": []string{"one", "one"}}}
		}},
		{"mixed_image_limit", func(r *GenerationRequest) {
			r.Images = []MediaReference{{URL: "https://cdn.example/image.png"}}
			r.ProviderOptions = map[string]map[string]any{"vinted-seedance-video": {"image_ids": make([]string, 9)}}
		}},
		{"invalid_face", func(r *GenerationRequest) {
			r.ProviderOptions = map[string]map[string]any{"vinted-seedance-video": {"native_face": "false"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := vintedTestRequest()
			tc.change(&request)
			if _, err := adapter.BuildCreate(context.Background(), RequestContext{Request: request}); err == nil {
				t.Fatal("unsupported input was accepted")
			}
		})
	}
}
