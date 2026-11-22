package main

import (
	"errors"
	"fmt"
	"path"
	"strings"
	"unicode/utf8"
)

const (
	maxAlbumTitleRunes = 120
	minAlbumPinRunes   = 4
	maxAlbumPinRunes   = 64
	maxPhotoNameRunes  = 180
	maxOriginalBytes   = 50 << 20
	maxWebBytes        = 12 << 20
	maxUploadBatch     = 20
	maxPhotoDimension  = 200_000
	photoStatusPending = "pending"
	photoStatusReady   = "ready"
	webJPEGContentType = "image/jpeg"
)

var (
	errTitleInvalid   = errors.New("title must be 1 to 120 characters")
	errPinInvalid     = errors.New("pin must be 4 to 64 characters")
	errPhotoType      = errors.New("use a JPEG, PNG, WebP, or HEIC photo")
	errPhotoTooLarge  = errors.New("each photo must be 50 MB or smaller")
	errWebTooLarge    = errors.New("web copy must be 12 MB or smaller")
	errPhotoEmpty     = errors.New("empty file")
	errWebEmpty       = errors.New("web copy is empty")
	errUploadBatch    = errors.New("upload at most 20 photos at a time")
	errVariantInvalid = errors.New("variant must be web or original")
)

type photoVariant int

const (
	variantWeb photoVariant = iota + 1
	variantOriginal
)

func parsePhotoVariant(raw string) (photoVariant, error) {
	switch strings.TrimSpace(raw) {
	case "web":
		return variantWeb, nil
	case "original":
		return variantOriginal, nil
	default:
		return 0, errVariantInvalid
	}
}

func photoObjectKey(photo photoStored, variant photoVariant) (string, error) {
	switch variant {
	case variantWeb:
		if photo.WebKey == "" {
			return "", errVariantInvalid
		}
		return photo.WebKey, nil
	case variantOriginal:
		if photo.OriginalKey == "" {
			return "", errVariantInvalid
		}
		return photo.OriginalKey, nil
	default:
		return "", fmt.Errorf("unhandled photo variant %d", variant)
	}
}

func downloadContentType(photo photoStored, variant photoVariant) string {
	switch variant {
	case variantWeb:
		return webJPEGContentType
	case variantOriginal:
		if photo.ContentType != "" {
			return photo.ContentType
		}
		return "application/octet-stream"
	default:
		return "application/octet-stream"
	}
}

func normalizeAlbumTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	n := utf8.RuneCountInString(title)
	if n < 1 || n > maxAlbumTitleRunes {
		return "", errTitleInvalid
	}
	return title, nil
}

func normalizeAlbumPin(pin string) (string, error) {
	pin = strings.TrimSpace(pin)
	n := utf8.RuneCountInString(pin)
	if n < minAlbumPinRunes || n > maxAlbumPinRunes {
		return "", errPinInvalid
	}
	return pin, nil
}

func classifyImage(contentType, filename string) (ext, canonical string, ok bool) {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	switch ct {
	case "image/jpeg", "image/jpg", "image/pjpeg":
		return ".jpg", webJPEGContentType, true
	case "image/png":
		return ".png", "image/png", true
	case "image/webp":
		return ".webp", "image/webp", true
	case "image/heic":
		return ".heic", "image/heic", true
	case "image/heif":
		return ".heif", "image/heif", true
	}

	switch strings.ToLower(path.Ext(filename)) {
	case ".jpg", ".jpeg":
		return ".jpg", webJPEGContentType, true
	case ".png":
		return ".png", "image/png", true
	case ".webp":
		return ".webp", "image/webp", true
	case ".heic":
		return ".heic", "image/heic", true
	case ".heif":
		return ".heif", "image/heif", true
	default:
		return "", "", false
	}
}

func cleanPhotoFilename(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = path.Base(name)
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || r == '"' || r == '\\' {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == ".." {
		return "photo"
	}
	if utf8.RuneCountInString(name) <= maxPhotoNameRunes {
		return name
	}
	runes := []rune(name)
	return string(runes[:maxPhotoNameRunes])
}

func downloadFilename(photo photoStored, variant photoVariant) string {
	base := cleanPhotoFilename(photo.Filename)
	ext := path.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	if stem == "" || stem == "." {
		stem = "photo"
	}
	switch variant {
	case variantWeb:
		return stem + ".jpg"
	case variantOriginal:
		if ext == "" {
			ext = path.Ext(photo.OriginalKey)
		}
		if ext == "" {
			ext = ".jpg"
		}
		return stem + ext
	default:
		return stem + ".jpg"
	}
}

func attachmentDisposition(filename string) string {
	return `attachment; filename="` + downloadFilenameSafe(filename) + `"`
}

func downloadFilenameSafe(filename string) string {
	filename = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 || r == '"' || r == '\\' {
			return -1
		}
		return r
	}, filename)
	if filename == "" {
		return "photo"
	}
	return filename
}

func originalObjectKey(albumID, photoID, ext string) string {
	return "albums/" + albumID + "/" + photoID + "/original" + ext
}

func webObjectKey(albumID, photoID string) string {
	return "albums/" + albumID + "/" + photoID + "/web.jpg"
}

func sharePath(token string) string {
	return "/gallery/" + token
}

func validObjectID(id string) bool {
	if len(id) != 16 && len(id) != 32 {
		return false
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
