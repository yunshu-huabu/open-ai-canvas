package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yingce/backend/internal/protocol"
)

const vintedTestMP4 = "\x00\x00\x00\x18ftypisom\x00\x00\x02\x00isomiso2"

func TestVintedModelCapabilitiesMatchPlugin(t *testing.T) {
	long := DefaultModelCapabilityConfigForModel("vinted-seedance-video", "seedance2.5").Video
	if long.Duration.Default != 30 || long.Duration.Min != 16 || long.References.MaxImages != 30 || long.References.MaxAudios != 10 || long.References.PromptMaxChars != 15000 || len(long.Resolutions) != 1 || long.Resolutions[0] != "720p" {
		t.Fatalf("long profile=%#v", long)
	}
	mini := DefaultModelCapabilityConfigForModel("vinted-seedance-video", "seedance2.0mini").Video
	if len(mini.Duration.Values) != 2 || mini.Duration.Values[1] != 10 || mini.References.MaxVideos != 3 {
		t.Fatalf("mini profile=%#v", mini)
	}
}

func vintedTestPreflight(w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case "/v1/catalog":
		_, _ = w.Write([]byte(`{"models":[{"id":"seedance2.0fast","durations":[5,10,15]}]}`))
	case "/v1/pricing":
		_, _ = w.Write([]byte(`{"items":[{"model":"seedance2.0fast","duration":5,"credits":10}],"native_face_surcharge_credits":5}`))
	case "/v1/balance":
		_, _ = w.Write([]byte(`{"balance":100}`))
	default:
		return false
	}
	return true
}

func TestVintedVideoPreparationAndSafeSubmissionRetry(t *testing.T) {
	allowLoopbackProviderTest(t)
	adapter := vintedPackageAdapter(t)
	uploads, creates := 0, 0
	var originalBody, operation string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if vintedTestPreflight(w, r) {
			return
		}
		switch r.URL.Path {
		case "/v1/files":
			uploads++
			var body map[string]string
			if json.NewDecoder(r.Body).Decode(&body) != nil || !strings.HasPrefix(body["image_b64"], "data:image/png;base64,") {
				t.Error("reference image was not uploaded as JSON base64")
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"image_id":"uploaded-1"}`))
		case "/v1/videos":
			creates++
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			encoded, _ := json.Marshal(body)
			if creates == 1 {
				originalBody, operation = string(encoded), r.Header.Get("Idempotency-Key")
				_, _ = w.Write([]byte(`null`))
				return
			}
			if operation == "" || operation != r.Header.Get("Idempotency-Key") || originalBody != string(encoded) {
				t.Error("submission retry changed operation or body")
			}
			ids, _ := body["image_ids"].([]any)
			if len(ids) != 1 || ids[0] != "uploaded-1" || len(body["video_b64s"].([]any)) != 1 || len(body["audio_b64s"].([]any)) != 1 {
				t.Errorf("references = %#v", body)
			}
			_, _ = w.Write([]byte(`{"id":"original-job","status":"failed","error":"mock final state"}`))
		default:
			t.Errorf("unexpected %s", r.URL.Path)
		}
	}))
	defer server.Close()
	config := providerConfig{BaseURL: server.URL, APIKey: "test-key", Model: "seedance2.0fast", InterfaceType: "vinted-seedance-video"}
	png := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jBz8AAAAASUVORK5CYII="
	input := canvasGenerationInput{Mode: "video", Prompt: "a clip", Config: config, ReferenceImages: []providerMedia{{DataURL: png}}, ReferenceVideos: []providerMedia{{DataURL: "data:video/mp4;base64," + base64.StdEncoding.EncodeToString([]byte(vintedTestMP4))}}, ReferenceAudios: []providerMedia{{DataURL: "data:audio/mpeg;base64," + base64.StdEncoding.EncodeToString([]byte("ID3\x00\x00\x00\x00\x00\x00\x00\x00\x00"))}}}
	_, err := runProtocolAdapterTaskWithPolicy(context.Background(), input, adapter, fastVideoPollPolicy())
	if err == nil || !strings.Contains(err.Error(), "mock final state") || creates != 2 || uploads != 1 {
		t.Fatalf("err=%v creates=%d uploads=%d", err, creates, uploads)
	}
}

func TestVintedVideoStopsBeforeChargingOnPreparationFailure(t *testing.T) {
	allowLoopbackProviderTest(t)
	adapter := vintedPackageAdapter(t)
	for _, stage := range []string{"catalog", "pricing", "balance", "files"} {
		t.Run(stage, func(t *testing.T) {
			creates := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/v1/"+stage {
					switch stage {
					case "catalog":
						_, _ = w.Write([]byte(`{"models":[]}`))
					case "pricing":
						_, _ = w.Write([]byte(`{"items":[]}`))
					case "balance":
						_, _ = w.Write([]byte(`{"balance":0}`))
					case "files":
						_, _ = w.Write([]byte(`{"image_id":""}`))
					}
					return
				}
				if vintedTestPreflight(w, r) {
					return
				}
				creates++
				t.Error("creation reached after a preparation failure")
			}))
			defer server.Close()
			config := providerConfig{BaseURL: server.URL, APIKey: "test-key", Model: "seedance2.0fast", InterfaceType: "vinted-seedance-video"}
			input := canvasGenerationInput{Mode: "video", Prompt: "a clip", Config: config, ReferenceImages: []providerMedia{{DataURL: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jBz8AAAAASUVORK5CYII="}}}
			if _, err := runProtocolAdapterTaskWithPolicy(context.Background(), input, adapter, fastVideoPollPolicy()); err == nil || creates != 0 {
				t.Fatalf("err=%v creates=%d", err, creates)
			}
		})
	}
}

func TestVintedOriginalDownloadResumesSameFile(t *testing.T) {
	allowLoopbackProviderTest(t)
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("Authorization") != "" {
			t.Error("signed download must be anonymous")
		}
		if requests == 1 {
			connection, buffer, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
				return
			}
			_, _ = fmt.Fprintf(buffer, "HTTP/1.1 200 OK\r\nContent-Type: video/mp4\r\nX-Video-Quality: original\r\nETag: original-file\r\nContent-Length: %d\r\n\r\n%s", len(vintedTestMP4), vintedTestMP4[:12])
			_ = buffer.Flush()
			_ = connection.Close()
			return
		}
		if r.Header.Get("Range") != "bytes=12-" || r.Header.Get("If-Range") != "original-file" {
			t.Error("resume headers missing")
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("X-Video-Quality", "original")
		w.Header().Set("ETag", "original-file")
		w.Header().Set("Content-Range", fmt.Sprintf("bytes 12-%d/%d", len(vintedTestMP4)-1, len(vintedTestMP4)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte(vintedTestMP4[12:]))
	}))
	defer server.Close()
	state := &protocolDownloadState{}
	config := providerConfig{BaseURL: server.URL, APIKey: "secret"}
	if _, _, err := downloadProtocolOriginal(context.Background(), config, server.URL+"/file", state); err == nil {
		t.Fatal("truncated download accepted")
	}
	data, _, err := downloadProtocolOriginal(context.Background(), config, server.URL+"/file?new-signature=1", state)
	if err != nil || !bytes.Equal(data, []byte(vintedTestMP4)) || requests != 2 {
		t.Fatalf("data=%q err=%v requests=%d", data, err, requests)
	}
}

func TestVintedOriginalRejectsPreviewHeaderAndFakeMP4(t *testing.T) {
	allowLoopbackProviderTest(t)
	for _, tc := range []struct{ name, quality, body string }{
		{"preview", "preview", vintedTestMP4}, {"missing_quality", "", vintedTestMP4}, {"html_disguised_as_mp4", "original", "<html>not a video</html>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "video/mp4")
				w.Header().Set("X-Video-Quality", tc.quality)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			if _, _, err := downloadProtocolOriginal(context.Background(), providerConfig{BaseURL: server.URL}, server.URL+"/file", &protocolDownloadState{}); err == nil {
				t.Fatal("unverified original was accepted")
			}
		})
	}
}

func vintedPackageAdapter(t *testing.T) protocol.Adapter {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "plugin-packages", "vinted-seedance-video.yingce-plugin"))
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := protocol.ParsePluginPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	adapter, err := protocol.LoadManifest(pkg.ManifestRaw)
	if err != nil {
		t.Fatal(err)
	}
	return adapter
}

func TestVintedVideoRuntimeDownloadsOriginalAndRefreshesSignature(t *testing.T) {
	allowLoopbackProviderTest(t)
	adapter := vintedPackageAdapter(t)
	for _, external := range []bool{false, true} {
		t.Run(fmt.Sprintf("external=%v", external), func(t *testing.T) {
			createCalls, pollCalls, signCalls, fileCalls := 0, 0, 0, 0
			media := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fileCalls++
				if external && (r.Header.Get("Authorization") != "" || r.Header.Get("X-Provider-Secret") != "") {
					t.Error("provider credentials leaked to download origin")
				}
				if r.URL.RawQuery != "signature=abc%2Bdef&version=original" {
					t.Errorf("signature query changed: %s", r.URL.RawQuery)
				}
				if fileCalls == 1 {
					w.WriteHeader(http.StatusServiceUnavailable)
					return
				}
				w.Header().Set("Content-Type", "video/mp4")
				w.Header().Set("X-Video-Quality", "original")
				_, _ = w.Write([]byte(vintedTestMP4))
			})
			cdn := httptest.NewServer(media)
			defer cdn.Close()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v1/videos/job-1/file" && r.Header.Get("Authorization") != "Bearer test-key" {
					t.Error("missing provider authentication")
				}
				w.Header().Set("Content-Type", "application/json")
				if vintedTestPreflight(w, r) {
					return
				}
				switch r.URL.Path {
				case "/v1/videos":
					createCalls++
					if r.Method != http.MethodPost || r.Header.Get("Idempotency-Key") == "" {
						t.Error("missing create method or idempotency key")
					}
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["duration"] != float64(5) {
						t.Errorf("create body = %#v, err = %v", body, err)
					}
					w.WriteHeader(http.StatusAccepted)
					_, _ = w.Write([]byte(`{"id":"job-1","status":"queued"}`))
				case "/v1/videos/job-1":
					pollCalls++
					if pollCalls == 1 {
						_, _ = w.Write([]byte(`{"status":"running","download_url":"https://cdn.invalid/preview.mp4"}`))
						return
					}
					_, _ = w.Write([]byte(`{"status":"succeeded","url":"https://cdn.invalid/preview.mp4"}`))
				case "/v1/videos/job-1/signed_url":
					signCalls++
					if r.URL.Query().Get("download") != "1" {
						t.Error("original download was not requested")
					}
					if signCalls == 1 {
						w.WriteHeader(http.StatusServiceUnavailable)
						return
					}
					url := "/v1/videos/job-1/file?signature=abc%2Bdef&version=original"
					if external {
						url = cdn.URL + url
					}
					_ = json.NewEncoder(w).Encode(map[string]any{"quality": "original", "url": url})
				case "/v1/videos/job-1/file":
					media.ServeHTTP(w, r)
				default:
					t.Errorf("unexpected path: %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			config := providerConfig{BaseURL: server.URL, APIKey: "test-key", Model: "seedance2.0fast", InterfaceType: "vinted-seedance-video", Headers: []OutboundHeader{{Name: "X-Provider-Secret", Value: "secret"}}}
			output, err := runProtocolAdapterTaskWithPolicy(context.Background(), canvasGenerationInput{Mode: "video", Prompt: "a clip", Config: config}, adapter, fastVideoPollPolicy())
			if err != nil {
				t.Fatal(err)
			}
			video, _ := output["video"].(map[string]interface{})
			if video["dataUrl"] != "data:video/mp4;base64,"+base64.StdEncoding.EncodeToString([]byte(vintedTestMP4)) || createCalls != 1 || pollCalls != 2 || signCalls != 3 || fileCalls != 2 {
				t.Fatalf("output=%#v create=%d poll=%d sign=%d file=%d", output, createCalls, pollCalls, signCalls, fileCalls)
			}
		})
	}
}

func TestVintedVideoRuntimeRejectsUnconfirmedOriginal(t *testing.T) {
	allowLoopbackProviderTest(t)
	adapter := vintedPackageAdapter(t)
	for _, response := range []string{`{"quality":"preview","url":"/preview.mp4"}`, `{"quality":"original"}`, `{"url":"/preview.mp4"}`} {
		t.Run(response, func(t *testing.T) {
			createCalls, downloadCalls := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if vintedTestPreflight(w, r) {
					return
				}
				switch r.URL.Path {
				case "/v1/videos":
					createCalls++
					w.WriteHeader(http.StatusAccepted)
					_, _ = w.Write([]byte(`{"id":"job-1","status":"succeeded"}`))
				case "/v1/videos/job-1/signed_url":
					_, _ = w.Write([]byte(response))
				default:
					downloadCalls++
					t.Errorf("unconfirmed media requested: %s", r.URL.Path)
				}
			}))
			defer server.Close()
			config := providerConfig{BaseURL: server.URL, APIKey: "test-key", Model: "seedance2.0fast", InterfaceType: "vinted-seedance-video"}
			output, err := runProtocolAdapterTaskWithPolicy(context.Background(), canvasGenerationInput{Mode: "video", Prompt: "a clip", Config: config}, adapter, fastVideoPollPolicy())
			if err == nil || !strings.Contains(err.Error(), "original") || output != nil || createCalls != 1 || downloadCalls != 0 {
				t.Fatalf("output=%#v err=%v create=%d download=%d", output, err, createCalls, downloadCalls)
			}
		})
	}
}
