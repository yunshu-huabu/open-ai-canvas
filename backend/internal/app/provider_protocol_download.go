package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type protocolResponseGuardKey struct{}
type signedMediaLinkError struct{ cause error }

func (e signedMediaLinkError) Error() string {
	return "原片签名已失效，将重新获取同一任务的签名：" + e.cause.Error()
}
func (e signedMediaLinkError) Unwrap() error { return e.cause }

type protocolDownloadState struct {
	data  []byte
	etag  string
	total int64
}

func downloadProtocolOriginal(ctx context.Context, config providerConfig, rawURL string, state *protocolDownloadState) ([]byte, string, error) {
	base, err := url.Parse(config.BaseURL)
	if err != nil {
		return nil, "", err
	}
	target, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.ResolveReference(target).String(), nil)
	if err != nil {
		return nil, "", err
	}
	// Signed delivery is anonymous, including same-origin files. Never forward channel credentials.
	req.Header.Set("User-Agent", "vinted-video-client/1.0")
	offset := len(state.data)
	if offset > 0 && state.etag != "" && state.total > int64(offset) {
		req.Header.Set("Range", "bytes="+strconv.Itoa(offset)+"-")
		req.Header.Set("If-Range", state.etag)
	} else {
		offset = 0
		state.data = nil
	}
	admitted := false
	guard := func(response *http.Response) error {
		mimeType := strings.ToLower(strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0]))
		if response.Header.Get("X-Video-Quality") != "original" || mimeType != "video/mp4" {
			return errors.New("原片响应未确认 original 质量或 MP4 格式")
		}
		if response.StatusCode == http.StatusPartialContent {
			var start, end, total int64
			_, scanErr := fmt.Sscanf(response.Header.Get("Content-Range"), "bytes %d-%d/%d", &start, &end, &total)
			if scanErr != nil || offset == 0 || start != int64(offset) || total != state.total || end != total-1 || response.Header.Get("ETag") != state.etag {
				return errors.New("原片续传的文件版本或字节范围不一致")
			}
		} else if response.StatusCode == http.StatusOK {
			state.data = nil
			state.etag, state.total = response.Header.Get("ETag"), response.ContentLength
		} else {
			return errors.New("原片下载返回非完整文件响应")
		}
		admitted = true
		return nil
	}
	req = req.WithContext(context.WithValue(req.Context(), protocolResponseGuardKey{}, guard))
	_, mimeType, err := doBinaryWithConsumer(req, func(_ string, chunk []byte) {
		if admitted {
			state.data = append(state.data, chunk...)
		}
	})
	if err != nil {
		var httpErr providerHTTPError
		if errors.As(err, &httpErr) && (httpErr.StatusCode == http.StatusForbidden || httpErr.StatusCode == http.StatusUnauthorized) {
			return nil, "", signedMediaLinkError{err}
		}
		return nil, "", err
	}
	if state.total > 0 && int64(len(state.data)) != state.total {
		return nil, "", errors.New("原片文件大小不完整")
	}
	if !protocolMediaFormatMatches("video/mp4", state.data) {
		return nil, "", errors.New("原片内容不是有效 MP4 文件")
	}
	return state.data, mimeType, nil
}
