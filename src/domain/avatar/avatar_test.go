package avatar

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
	"time"
)

func TestProcessCanonicalizesUpload(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 80, 40))
	for y := 0; y < 40; y++ {
		for x := 0; x < 80; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 3), G: uint8(y * 5), B: 90, A: 255})
		}
	}
	var upload bytes.Buffer
	if err := png.Encode(&upload, source); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.August, 15, 12, 0, 0, 123456789, time.FixedZone("test", 2*60*60))
	processed, err := Process(42, upload.Bytes(), now)
	if err != nil {
		t.Fatal(err)
	}
	if processed.UserID != 42 || processed.ContentType != ContentType || processed.Width != OutputSize || processed.Height != OutputSize || processed.ByteSize != int64(len(processed.Data)) || len(processed.ETag) != 64 {
		t.Fatalf("unexpected canonical avatar: %+v", processed)
	}
	if processed.UpdatedAt.Location() != time.UTC || !processed.UpdatedAt.Equal(now.UTC().Truncate(time.Millisecond)) {
		t.Fatalf("timestamp was not normalized: %v", processed.UpdatedAt)
	}
	configuration, err := jpeg.DecodeConfig(bytes.NewReader(processed.Data))
	if err != nil || configuration.Width != OutputSize || configuration.Height != OutputSize {
		t.Fatalf("output is not a 512px JPEG: config=%+v err=%v", configuration, err)
	}
	if err := processed.ValidateStored(); err != nil {
		t.Fatalf("canonical avatar did not validate: %v", err)
	}
}

func TestProcessFlattensTransparentPixels(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: 255, A: 0})
		}
	}
	var upload bytes.Buffer
	if err := png.Encode(&upload, source); err != nil {
		t.Fatal(err)
	}
	processed, err := Process(42, upload.Bytes(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(processed.Data))
	if err != nil {
		t.Fatal(err)
	}
	r, g, b, a := decoded.At(256, 256).RGBA()
	if a != 0xffff || r < 0xef00 || g < 0xef00 || b < 0xef00 {
		t.Fatalf("transparent source was not flattened onto a light opaque background: rgba=%04x,%04x,%04x,%04x", r, g, b, a)
	}
}

func TestProcessRejectsUntrustedInputsBeforeDecode(t *testing.T) {
	tooLarge := make([]byte, MaxUploadBytes+1)
	if _, err := Process(42, tooLarge, time.Now()); !errors.Is(err, ErrImageTooLarge) {
		t.Fatalf("oversized upload returned %v", err)
	}
	if _, err := Process(42, []byte("not an image"), time.Now()); !errors.Is(err, ErrUnsupportedFormat) {
		t.Fatalf("unsupported upload returned %v", err)
	}
	truncatedPNG := []byte("\x89PNG\r\n\x1a\n")
	if _, err := Process(42, truncatedPNG, time.Now()); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("truncated PNG returned %v", err)
	}
	if _, err := Process(0, truncatedPNG, time.Now()); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("invalid user ID returned %v", err)
	}
}

func TestDimensionsBoundDecodedMemory(t *testing.T) {
	for _, dimensions := range [][2]int{{0, 100}, {4097, 1}, {4096, 4000}} {
		if dimensionsAllowed(dimensions[0], dimensions[1]) {
			t.Fatalf("unsafe dimensions were accepted: %dx%d", dimensions[0], dimensions[1])
		}
	}
	if !dimensionsAllowed(4000, 4000) {
		t.Fatal("16 million pixels should be accepted")
	}
}

func TestValidateStoredDetectsTampering(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var upload bytes.Buffer
	if err := jpeg.Encode(&upload, source, nil); err != nil {
		t.Fatal(err)
	}
	processed, err := Process(42, upload.Bytes(), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	processed.Data[0] ^= 0xff
	if err := processed.ValidateStored(); !errors.Is(err, ErrInvalidImage) {
		t.Fatalf("tampered bytes returned %v", err)
	}
}
