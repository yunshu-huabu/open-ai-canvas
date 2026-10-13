package app

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
	"yingce/backend/internal/assets"
	"yingce/backend/internal/model"
)

// Local static images are prepared once, using the existing thumbnail columns.
// The storage lock also prevents deletion while the derivative is published.
func (s *Service) ensureLocalResourceThumbnail(resource *model.Resource) error {
	if resource.Provider != "local" || resource.Status != model.ResourceStatusReady || resource.Kind != "image" ||
		(resource.MimeType != "image/png" && resource.MimeType != "image/jpeg") || resource.ThumbnailStatus == "ready" {
		return nil
	}
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	current, err := s.repo.ResourceForUser(resource.UserID, resource.ID)
	if err != nil {
		return err
	}
	if current.Status != model.ResourceStatusReady {
		return errors.New("资源尚未就绪")
	}
	if current.ThumbnailStatus == "ready" {
		*resource = *current
		return nil
	}
	file, err := os.Open(filepath.Join(s.dataDir, "resources", filepath.FromSlash(current.ObjectKey)))
	if err != nil {
		return err
	}
	defer file.Close()
	config, format, err := image.DecodeConfig(file)
	if err != nil {
		return err
	}
	if (format != "png" && format != "jpeg") || config.Width <= 0 || config.Height <= 0 || config.Width > 64_000_000/config.Height {
		return errors.New("图片格式或像素数不支持缩略图")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	source, _, err := image.Decode(file)
	if err != nil {
		return err
	}
	width, height := config.Width, config.Height
	if width > assets.ThumbnailMaxEdge || height > assets.ThumbnailMaxEdge {
		if width >= height {
			height = max(1, height*assets.ThumbnailMaxEdge/width)
			width = assets.ThumbnailMaxEdge
		} else {
			width = max(1, width*assets.ThumbnailMaxEdge/height)
			height = assets.ThumbnailMaxEdge
		}
	}
	thumbnail := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(thumbnail, thumbnail.Bounds(), source, source.Bounds(), draw.Src, nil)
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, thumbnail); err != nil {
		return err
	}
	objectKey := assets.ThumbnailObjectKey(current.ObjectKey)
	size := int64(encoded.Len())
	if err := writeLocalResourceObject(filepath.Join(s.dataDir, "resources", filepath.FromSlash(objectKey)), &encoded); err != nil {
		return err
	}
	updated, err := s.repo.UpdateResourceThumbnail(current.UserID, current.ID, map[string]any{
		"thumbnail_status": "ready", "thumbnail_mime_type": "image/png", "thumbnail_size": size,
		"thumbnail_width": width, "thumbnail_height": height, "width": config.Width, "height": config.Height,
	})
	if err != nil || !updated {
		_ = s.deleteLocalResourceObject(objectKey)
		if err != nil {
			return err
		}
		return NotFound("资源不存在")
	}
	current.ThumbnailStatus, current.ThumbnailMimeType, current.ThumbnailSize = "ready", "image/png", size
	current.ThumbnailWidth, current.ThumbnailHeight = width, height
	current.Width, current.Height = config.Width, config.Height
	*resource = *current
	return nil
}
