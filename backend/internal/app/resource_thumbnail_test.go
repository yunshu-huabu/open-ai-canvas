package app

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yingce/backend/internal/assets"
	"yingce/backend/internal/model"
)

func TestLocalResourceThumbnailColdAccessAndReuse(t *testing.T) {
	svc, _, _ := newResourceDeletionTestService(t)
	var original bytes.Buffer
	if err := png.Encode(&original, image.NewNRGBA(image.Rect(0, 0, 1920, 1080))); err != nil {
		t.Fatal(err)
	}
	resource := &model.Resource{ID: "thumbnail-image", UserID: "user-1", Kind: "image", Status: model.ResourceStatusReady,
		Provider: "local", ObjectKey: "image/test.png", MimeType: "image/png", Size: int64(original.Len())}
	if err := writeLocalResourceObject(filepath.Join(svc.dataDir, "resources", resource.ObjectKey), bytes.NewReader(original.Bytes())); err != nil {
		t.Fatal(err)
	}
	if err := svc.repo.CreateResource(resource); err != nil {
		t.Fatal(err)
	}
	options := ResourceAccessOptions{Purpose: assets.PurposeDisplay, Variant: assets.VariantThumbnail}
	access, err := svc.ResourceAccessBatch(resource.UserID, []ResourceAccessRequest{{ResourceID: resource.ID, AccessOptions: options}})
	if err != nil || len(access) != 1 || access[0].Access == nil {
		t.Fatalf("cold access = %#v, %v", access, err)
	}
	if access[0].Access.ActualVariant != assets.VariantThumbnail || access[0].Access.OriginalWidth != 1920 || access[0].Access.OriginalHeight != 1080 {
		t.Fatalf("thumbnail access = %#v", access[0].Access)
	}
	delivery, err := svc.PrepareResourceDelivery(resource.UserID, resource.ID, options, "")
	if err != nil {
		t.Fatal(err)
	}
	config, _, err := image.DecodeConfig(delivery.Stream.Body)
	delivery.Stream.Body.Close()
	if err != nil || config.Width != 512 || config.Height != 288 {
		t.Fatalf("thumbnail dimensions = %#v, %v", config, err)
	}
	filePath := filepath.Join(svc.dataDir, "resources", assets.ThumbnailObjectKey(resource.ObjectKey))
	before, err := os.Stat(filePath)
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.ResourceAccessBatch(resource.UserID, []ResourceAccessRequest{{ResourceID: resource.ID, AccessOptions: options}})
	if err != nil || second[0].Access.URL != access[0].Access.URL {
		t.Fatalf("cached access = %#v, %v", second, err)
	}
	after, _ := os.Stat(filePath)
	if !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("warm access rewrote the thumbnail")
	}
	originalDelivery, err := svc.PrepareResourceDelivery(resource.UserID, resource.ID, ResourceAccessOptions{Purpose: assets.PurposeDownload, Variant: assets.VariantThumbnail}, "")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(originalDelivery.Stream.Body)
	originalDelivery.Stream.Body.Close()
	if err != nil || !bytes.Equal(raw, original.Bytes()) || originalDelivery.Access.ActualVariant != assets.VariantOriginal {
		t.Fatalf("download did not preserve original: %v", err)
	}
	denied, err := svc.ResourceAccessBatch("another-user", []ResourceAccessRequest{{ResourceID: resource.ID, AccessOptions: options}})
	if err != nil || denied[0].Access != nil || denied[0].Error == nil {
		t.Fatalf("ownership not enforced: %#v, %v", denied, err)
	}
	if err := svc.deleteStoredResourceObject(resource.UserID, resource); err != nil {
		t.Fatal(err)
	}
	for _, objectKey := range []string{resource.ObjectKey, assets.ThumbnailObjectKey(resource.ObjectKey)} {
		if _, err := os.Stat(filepath.Join(svc.dataDir, "resources", objectKey)); !os.IsNotExist(err) {
			t.Fatalf("resource object was not deleted: %s, %v", objectKey, err)
		}
	}
}

func TestLocalResourceThumbnailRejectsOversizedHeaderBeforeDecode(t *testing.T) {
	svc := newResourceTestService(t)
	header := make([]byte, 33)
	copy(header, "\x89PNG\r\n\x1a\n")
	binary.BigEndian.PutUint32(header[8:12], 13)
	copy(header[12:16], "IHDR")
	binary.BigEndian.PutUint32(header[16:20], 100_000)
	binary.BigEndian.PutUint32(header[20:24], 100_000)
	header[24], header[25] = 8, 6
	binary.BigEndian.PutUint32(header[29:33], crc32.ChecksumIEEE(header[12:29]))
	resource := &model.Resource{ID: "oversized", UserID: "user-1", Kind: "image", Status: model.ResourceStatusReady,
		Provider: "local", ObjectKey: "image/oversized.png", MimeType: "image/png", Size: int64(len(header))}
	if err := writeLocalResourceObject(filepath.Join(svc.dataDir, "resources", resource.ObjectKey), bytes.NewReader(header)); err != nil {
		t.Fatal(err)
	}
	if err := svc.repo.CreateResource(resource); err != nil {
		t.Fatal(err)
	}
	err := svc.ensureLocalResourceThumbnail(resource)
	if err == nil || !strings.Contains(err.Error(), "像素数") {
		t.Fatalf("oversized header not rejected before pixel allocation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(svc.dataDir, "resources", assets.ThumbnailObjectKey(resource.ObjectKey))); !os.IsNotExist(err) {
		t.Fatalf("invalid thumbnail was published: %v", err)
	}
}
