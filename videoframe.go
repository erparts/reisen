package reisen

// #cgo pkg-config: libavutil libavformat libavcodec  libswscale
// #include <libavcodec/avcodec.h>
// #include <libavformat/avformat.h>
// #include <libavutil/avutil.h>
// #include <libavutil/imgutils.h>
// #include <libswscale/swscale.h>
// #include <inttypes.h>
import "C"
import "image"

// VideoFrame represents a single frame of a video stream.
type VideoFrame struct {
	baseFrame
	img *image.RGBA
}

// newVideoFrame creates a new video frame.
func newVideoFrame(stream Stream, pts int64, indCoded, indDisplay, width, height int, pix []byte) *VideoFrame {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	copy(img.Pix, pix)

	return &VideoFrame{
		baseFrame: baseFrame{
			stream:       stream,
			pts:          pts,
			indexCoded:   indCoded,
			indexDisplay: indDisplay,
		},
		img: img,
	}
}

// Data returns a byte slice of RGBA pixels of the frame image.
func (f *VideoFrame) Data() []byte {
	return f.img.Pix
}

// Image returns the RGBA image of the frame.
func (f *VideoFrame) Image() *image.RGBA {
	return f.img
}
