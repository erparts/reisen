package reisen

// #cgo pkg-config: libavutil libavformat libavcodec  libswscale
// #include <libavcodec/avcodec.h>
// #include <libavformat/avformat.h>
// #include <libavutil/avutil.h>
// #include <libavutil/imgutils.h>
// #include <libswscale/swscale.h>
// #include <inttypes.h>
import "C"
import (
	"time"
)

// Frame is an abstract data frame.
type Frame interface {
	Data() []byte
	PresentationOffset() (time.Duration, error)
}

// baseFrame contains the information common for all frames of any type.
type baseFrame struct {
	stream       Stream
	pts          int64
	indexCoded   int
	indexDisplay int
}

// PresentationOffset returns the duration offset since the start of the media
// at which the frame should be played.
func (s *baseFrame) PresentationOffset() (time.Duration, error) {
	tbNum, tbDen := s.stream.TimeBase()
	return time.Second * time.Duration(tbNum) * time.Duration(s.pts) / time.Duration(tbDen), nil
}

// IndexCoded returns the index of the frame in the bitstream order.
func (f *baseFrame) IndexCoded() int {
	return f.indexCoded
}

// IndexDisplay returns the index of the frame in the display order.
func (f *baseFrame) IndexDisplay() int {
	return f.indexDisplay
}
