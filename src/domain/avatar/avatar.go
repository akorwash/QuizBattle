package avatar

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	_ "image/png"
	"math"
	"net/http"
	"time"
)

const (
	MaxUploadBytes = 2 << 20
	MaxDimension   = 4096
	MaxPixels      = 16_000_000
	OutputSize     = 512
	JPEGQuality    = 88
	ContentType    = "image/jpeg"
)

var (
	ErrInvalidImage      = errors.New("avatar: invalid image")
	ErrImageTooLarge     = errors.New("avatar: image exceeds 2 MiB")
	ErrUnsupportedFormat = errors.New("avatar: only JPEG and PNG are supported")
	ErrInvalidDimensions = errors.New("avatar: image dimensions are not allowed")
)

// Image is the canonical avatar persisted by the application. User uploads
// are never stored verbatim: Process strips metadata, crops the image to a
// square and re-encodes it to a bounded, opaque JPEG.
type Image struct {
	UserID      int64     `bson:"userId"`
	ContentType string    `bson:"contentType"`
	Data        []byte    `bson:"data"`
	ETag        string    `bson:"etag"`
	Width       int       `bson:"width"`
	Height      int       `bson:"height"`
	ByteSize    int64     `bson:"byteSize"`
	UpdatedAt   time.Time `bson:"updatedAt"`
}

// Process validates an untrusted upload before performing a full decode. The
// returned bytes contain no EXIF, comments, ICC profiles or PNG ancillary
// chunks from the source file.
func Process(userID int64, upload []byte, updatedAt time.Time) (*Image, error) {
	if userID <= 0 || len(upload) == 0 {
		return nil, ErrInvalidImage
	}
	if len(upload) > MaxUploadBytes {
		return nil, ErrImageTooLarge
	}

	detectedType := http.DetectContentType(upload)
	if detectedType != "image/jpeg" && detectedType != "image/png" {
		return nil, ErrUnsupportedFormat
	}
	configuration, format, err := image.DecodeConfig(bytes.NewReader(upload))
	if err != nil {
		return nil, fmt.Errorf("%w: decode configuration", ErrInvalidImage)
	}
	if (format != "jpeg" || detectedType != "image/jpeg") && (format != "png" || detectedType != "image/png") {
		return nil, ErrUnsupportedFormat
	}
	if !dimensionsAllowed(configuration.Width, configuration.Height) {
		return nil, ErrInvalidDimensions
	}

	source, decodedFormat, err := image.Decode(bytes.NewReader(upload))
	if err != nil {
		return nil, fmt.Errorf("%w: decode pixels", ErrInvalidImage)
	}
	if decodedFormat != format {
		return nil, ErrInvalidImage
	}
	bounds := source.Bounds()
	if bounds.Dx() != configuration.Width || bounds.Dy() != configuration.Height || !dimensionsAllowed(bounds.Dx(), bounds.Dy()) {
		return nil, ErrInvalidDimensions
	}

	canonical := centerCropAndResize(source)
	var output bytes.Buffer
	if err := jpeg.Encode(&output, canonical, &jpeg.Options{Quality: JPEGQuality}); err != nil {
		return nil, fmt.Errorf("encode avatar: %w", err)
	}
	digest := sha256.Sum256(output.Bytes())
	data := append([]byte(nil), output.Bytes()...)
	return &Image{
		UserID:      userID,
		ContentType: ContentType,
		Data:        data,
		ETag:        hex.EncodeToString(digest[:]),
		Width:       OutputSize,
		Height:      OutputSize,
		ByteSize:    int64(len(data)),
		// MongoDB dates have millisecond precision. Canonicalizing here keeps the
		// upload response and a subsequent read byte-for-byte consistent.
		UpdatedAt: updatedAt.UTC().Truncate(time.Millisecond),
	}, nil
}

// ValidateStored prevents malformed or partially written database records
// from being served as trusted image content.
func (avatar *Image) ValidateStored() error {
	if avatar == nil || avatar.UserID <= 0 || avatar.ContentType != ContentType || len(avatar.Data) == 0 ||
		avatar.Width != OutputSize || avatar.Height != OutputSize || avatar.ByteSize != int64(len(avatar.Data)) || avatar.UpdatedAt.IsZero() {
		return ErrInvalidImage
	}
	digest := sha256.Sum256(avatar.Data)
	if avatar.ETag == "" || avatar.ETag != hex.EncodeToString(digest[:]) {
		return ErrInvalidImage
	}
	return nil
}

func dimensionsAllowed(width, height int) bool {
	if width <= 0 || height <= 0 || width > MaxDimension || height > MaxDimension {
		return false
	}
	return int64(width)*int64(height) <= MaxPixels
}

func centerCropAndResize(source image.Image) *image.NRGBA {
	bounds := source.Bounds()
	squareSize := min(bounds.Dx(), bounds.Dy())
	cropX := bounds.Min.X + (bounds.Dx()-squareSize)/2
	cropY := bounds.Min.Y + (bounds.Dy()-squareSize)/2
	destination := image.NewNRGBA(image.Rect(0, 0, OutputSize, OutputSize))

	for y := range OutputSize {
		sourceY := float64(cropY) + (float64(y)+0.5)*float64(squareSize)/OutputSize - 0.5
		for x := range OutputSize {
			sourceX := float64(cropX) + (float64(x)+0.5)*float64(squareSize)/OutputSize - 0.5
			destination.SetNRGBA(x, y, bilinearOpaque(source, sourceX, sourceY, bounds))
		}
	}
	return destination
}

func bilinearOpaque(source image.Image, sourceX, sourceY float64, bounds image.Rectangle) color.NRGBA {
	rawX0 := int(math.Floor(sourceX))
	rawY0 := int(math.Floor(sourceY))
	x0 := clamp(rawX0, bounds.Min.X, bounds.Max.X-1)
	y0 := clamp(rawY0, bounds.Min.Y, bounds.Max.Y-1)
	x1 := clamp(rawX0+1, bounds.Min.X, bounds.Max.X-1)
	y1 := clamp(rawY0+1, bounds.Min.Y, bounds.Max.Y-1)
	weightX := sourceX - math.Floor(sourceX)
	weightY := sourceY - math.Floor(sourceY)

	topLeft := opaquePixel(source.At(x0, y0))
	topRight := opaquePixel(source.At(x1, y0))
	bottomLeft := opaquePixel(source.At(x0, y1))
	bottomRight := opaquePixel(source.At(x1, y1))
	return color.NRGBA{
		R: interpolateChannel(topLeft.R, topRight.R, bottomLeft.R, bottomRight.R, weightX, weightY),
		G: interpolateChannel(topLeft.G, topRight.G, bottomLeft.G, bottomRight.G, weightX, weightY),
		B: interpolateChannel(topLeft.B, topRight.B, bottomLeft.B, bottomRight.B, weightX, weightY),
		A: 255,
	}
}

func opaquePixel(value color.Color) color.NRGBA {
	pixel := color.NRGBAModel.Convert(value).(color.NRGBA)
	if pixel.A == 255 {
		return pixel
	}
	// A light neutral background avoids turning transparent PNG regions black
	// when the canonical JPEG is rendered in the profile UI.
	const background = uint32(246)
	alpha := uint32(pixel.A)
	return color.NRGBA{
		// #nosec G115 -- each alpha blend is mathematically bounded to 0..255.
		R: uint8((uint32(pixel.R)*alpha + background*(255-alpha) + 127) / 255),
		// #nosec G115 -- each alpha blend is mathematically bounded to 0..255.
		G: uint8((uint32(pixel.G)*alpha + background*(255-alpha) + 127) / 255),
		// #nosec G115 -- each alpha blend is mathematically bounded to 0..255.
		B: uint8((uint32(pixel.B)*alpha + background*(255-alpha) + 127) / 255),
		A: 255,
	}
}

func interpolateChannel(topLeft, topRight, bottomLeft, bottomRight uint8, weightX, weightY float64) uint8 {
	top := float64(topLeft)*(1-weightX) + float64(topRight)*weightX
	bottom := float64(bottomLeft)*(1-weightX) + float64(bottomRight)*weightX
	value := top*(1-weightY) + bottom*weightY
	return uint8(math.Round(min(255, max(0, value))))
}

func clamp(value, minimum, maximum int) int {
	return min(max(value, minimum), maximum)
}
