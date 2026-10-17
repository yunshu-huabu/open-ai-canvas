package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"image/png"
	"slices"
	"sort"
	"strings"

	"golang.org/x/image/webp"

	"yingce/backend/internal/protocol"
)

type protocolReadLimitKey struct{}

func prepareProtocolRequest(ctx context.Context, input canvasGenerationInput, adapter protocol.Adapter, request *protocol.GenerationRequest) error {
	lifecycle := adapter.Metadata().Lifecycle
	if lifecycle == nil {
		return nil
	}
	for kind, limit := range lifecycle.MediaLimits {
		var references *[]protocol.MediaReference
		switch kind {
		case "images":
			references = &request.Images
		case "videos":
			references = &request.Videos
		case "audios":
			references = &request.Audios
		}
		var total int64
		for i := range *references {
			media := &(*references)[i]
			var data []byte
			var mimeType string
			var err error
			if media.DataURL != "" {
				mimeType, data, err = decodeProviderDataURL(media.DataURL)
			} else if media.URL != "" {
				data, mimeType, err = getExternalBinary(context.WithValue(ctx, protocolReadLimitKey{}, limit.FileBytes), media.URL)
			} else {
				return errors.New("参考素材为空")
			}
			if err != nil {
				return fmt.Errorf("读取参考素材失败：%w", err)
			}
			mimeType = strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
			if mimeType == "audio/x-wav" || mimeType == "audio/wave" {
				mimeType = "audio/wav"
			}
			if !slices.Contains(limit.MIMETypes, mimeType) || !protocolMediaFormatMatches(mimeType, data) {
				mimeType = ""
				for _, candidate := range limit.MIMETypes {
					if protocolMediaFormatMatches(candidate, data) {
						mimeType = candidate
						break
					}
				}
			}
			if mimeType == "" {
				return fmt.Errorf("%s 参考素材格式不支持或内容无效", kind)
			}
			if len(data) == 0 || int64(len(data)) > limit.FileBytes {
				return fmt.Errorf("%s 单个参考素材超过大小限制", kind)
			}
			total += int64(len(data))
			if total > limit.TotalBytes {
				return fmt.Errorf("%s 参考素材合计超过大小限制", kind)
			}
			media.DataURL, media.MIMEType = dataURL(mimeType, data), mimeType
		}
	}
	preparer, ok := adapter.(protocol.PreparationAdapter)
	if len(lifecycle.Prepare) > 0 && !ok {
		return errors.New("插件宿主不支持生成前准备")
	}
	values := map[string]any{}
	request.Extra["preparation"] = values
	defer delete(request.Extra, "prepareItem")
	for _, step := range lifecycle.Prepare {
		count := 1
		var images []protocol.MediaReference
		if step.Source == "images" {
			images = append(images, request.Images...)
			sort.SliceStable(images, func(i, j int) bool { return images[i].Order < images[j].Order })
			count = len(images)
		}
		items := make([]any, 0, count)
		for i := 0; i < count; i++ {
			if step.Source == "images" {
				request.Extra["prepareItem"] = map[string]any{"dataUrl": images[i].DataURL, "url": images[i].URL}
			}
			spec, err := preparer.BuildPreparation(ctx, step, *request)
			if err != nil {
				return err
			}
			body, err := executeProtocolRequest(withProviderRequestKind(ctx, "prepare"), input.Config, spec)
			if err != nil {
				return err
			}
			value, message, err := preparer.ParsePreparation(ctx, step, *request, body)
			if err != nil {
				return err
			}
			items = append(items, value)
			if message != "" {
				metadata, _ := ctx.Value(providerAnalyticsKey{}).(providerAnalyticsContext)
				if metadata.Service != nil && metadata.TaskID != "" {
					_ = metadata.Service.log(metadata.UserID, metadata.TaskID, "info", message, "")
				}
			}
		}
		if step.Source == "" {
			values[step.ID] = items[0]
		} else {
			values[step.ID] = items
		}
	}
	return nil
}

func protocolMediaFormatMatches(mimeType string, data []byte) bool {
	if strings.HasPrefix(mimeType, "image/") {
		switch mimeType {
		case "image/png":
			_, err := png.DecodeConfig(bytes.NewReader(data))
			return err == nil
		case "image/jpeg":
			_, err := jpeg.DecodeConfig(bytes.NewReader(data))
			return err == nil
		case "image/webp":
			_, err := webp.DecodeConfig(bytes.NewReader(data))
			return err == nil
		}
		return false
	}
	if len(data) < 12 {
		return false
	}
	switch mimeType {
	case "video/mp4", "video/quicktime", "audio/mp4", "audio/x-m4a":
		return string(data[4:8]) == "ftyp" || (mimeType == "video/quicktime" && slices.Contains([]string{"moov", "mdat", "wide"}, string(data[4:8])))
	case "video/webm":
		return bytes.HasPrefix(data, []byte{0x1a, 0x45, 0xdf, 0xa3})
	case "audio/mpeg":
		return bytes.HasPrefix(data, []byte("ID3")) || (data[0] == 0xff && data[1]&0xe0 == 0xe0)
	case "audio/wav":
		return string(data[:4]) == "RIFF" && string(data[8:12]) == "WAVE"
	case "audio/ogg", "audio/flac":
		return bytes.HasPrefix(data, []byte("OggS")) || (mimeType == "audio/flac" && bytes.HasPrefix(data, []byte("fLaC")))
	}
	return false
}

func executeProtocolCreateWithRetry(ctx context.Context, input canvasGenerationInput, adapter protocol.Adapter, spec protocol.RequestSpec, request protocol.GenerationRequest, policy videoPollPolicy) (protocol.CreateResult, error) {
	attempts := 1
	if lifecycle := adapter.Metadata().Lifecycle; lifecycle != nil && lifecycle.RequestBytes > 0 {
		encoded, err := json.Marshal(spec.Body)
		if err != nil {
			return protocol.CreateResult{}, err
		}
		if int64(len(encoded)) > lifecycle.RequestBytes {
			return protocol.CreateResult{}, errors.New("视频请求体超过上游限制，请减少参考音视频")
		}
	}
	if lifecycle := adapter.Metadata().Lifecycle; lifecycle != nil && lifecycle.CreateAttempts > 1 {
		if spec.Headers["Idempotency-Key"] == "" {
			return protocol.CreateResult{}, errors.New("创建重试缺少幂等键")
		}
		attempts = lifecycle.CreateAttempts
	}
	for attempt := 0; attempt < attempts; attempt++ {
		body, streamed, err := executeProtocolCreateRequest(withProviderRequestKind(ctx, "create"), input, spec)
		var result protocol.CreateResult
		if err == nil {
			if streamed != nil {
				return protocol.CreateResult{Status: protocol.StatusSucceeded, Result: streamed}, nil
			}
			if parser, ok := adapter.(protocol.RequestAwareCreateParser); ok {
				result, err = parser.ParseCreateWithRequest(ctx, request, body)
			} else {
				result, err = adapter.ParseCreate(ctx, body)
			}
			if err == nil && result.TaskID == "" {
				result.TaskID, err = extractProviderTaskID(body)
			}
			if err == nil && result.TaskID == "" && result.Result == nil && (attempts > 1 || (result.Status != protocol.StatusFailed && result.Status != protocol.StatusCancelled)) {
				err = providerResponseDecodeError{Err: errors.New("创建响应缺少任务 ID")}
			}
		}
		if err == nil {
			return result, nil
		}
		retry, _ := retryableVideoPollError(ctx, err)
		if attempt+1 == attempts || !retry {
			return protocol.CreateResult{}, err
		}
		if err := policy.Sleep(ctx, max(policy.Interval, providerRetryAfter(err))); err != nil {
			return protocol.CreateResult{}, err
		}
	}
	return protocol.CreateResult{}, errors.New("创建视频重试已结束")
}
