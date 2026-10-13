package assets

const ThumbnailMaxEdge = 512

func ThumbnailObjectKey(objectKey string) string {
	return objectKey + ".thumbnail.png"
}
