package wechat

import (
	"errors"
	"fmt"
	"io"
)

// ErrImageTooLarge indicates the input image exceeds WeChat API limits.
// Use errors.Is(err, ErrImageTooLarge) to detect.
var ErrImageTooLarge = errors.New("image too large")

const (
	// MaxImgSecCheckBytes is the upper bound we allow reading into memory for
	// image content safety checks.
	//
	// WeChat "多媒体内容安全识别" (media_check_async v2.0) documents a single file
	// size limit of 10M. Keep this aligned to avoid rejecting valid images
	// like ~1.1MB.
	MaxImgSecCheckBytes int64 = 10 * 1024 * 1024

	// MaxOCRImageBytes is a conservative limit for CV OCR endpoints.
	// Keep it modest to avoid memory spikes; callers may further constrain.
	MaxOCRImageBytes int64 = 2 * 1024 * 1024
)

func readAllLimited(r io.Reader, max int64) ([]byte, error) {
	if max <= 0 {
		return io.ReadAll(r)
	}
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("%w: max=%d bytes", ErrImageTooLarge, max)
	}
	return data, nil
}
