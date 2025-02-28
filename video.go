package reisen

// #cgo pkg-config: libavutil libavformat libavcodec libswscale
// #include <libavcodec/avcodec.h>
// #include <libavformat/avformat.h>
// #include <libavutil/avutil.h>
// #include <libavutil/imgutils.h>
// #include <libswscale/swscale.h>
// #include <inttypes.h>
import "C"
import (
	"fmt"
	"unsafe"
)

// VideoStream is a streaming holding video frames.
type VideoStream struct {
	baseStream
	swsCtx    *C.struct_SwsContext
	rgbaFrame *C.AVFrame
	bufSize   C.int
}

// AspectRatio returns the fraction of the video stream frame aspect ratio (1/0 if unknown).
func (s *VideoStream) AspectRatio() (int, int) {
	return int(s.params.sample_aspect_ratio.num),
		int(s.params.sample_aspect_ratio.den)
}

// Width returns the width of the video stream frame.
func (s *VideoStream) Width() int {
	return int(s.params.width)
}

// Height returns the height of the video stream frame.
func (s *VideoStream) Height() int {
	return int(s.params.height)
}

// OpenDecode opens the video stream for decoding with default parameters.
func (s *VideoStream) Open() error {
	return s.OpenDecode(
		int(s.params.width),
		int(s.params.height),
		InterpolationBicubic)
}

// OpenDecode opens the video stream for
// decoding with the specified parameters.
func (s *VideoStream) OpenDecode(width, height int, alg InterpolationAlgorithm) error {
	if err := s.open(); err != nil {
		return err
	}

	s.rgbaFrame = C.av_frame_alloc()
	if s.rgbaFrame == nil {
		return fmt.Errorf("couldn't allocate a new RGBA frame")
	}

	s.bufSize = C.av_image_get_buffer_size(
		C.AV_PIX_FMT_RGBA, C.int(width), C.int(height), 1)
	if s.bufSize < 0 {
		C.av_frame_free(&s.rgbaFrame)
		return fmt.Errorf("%d: couldn't get the buffer size", s.bufSize)
	}

	buf := (*C.uint8_t)(unsafe.Pointer(
		C.av_malloc(bufferSize(s.bufSize))))
	if buf == nil {
		C.av_frame_free(&s.rgbaFrame)
		return fmt.Errorf("couldn't allocate an AV buffer")
	}

	status := C.av_image_fill_arrays(&s.rgbaFrame.data[0],
		&s.rgbaFrame.linesize[0], buf, C.AV_PIX_FMT_RGBA,
		C.int(width), C.int(height), 1)
	if status < 0 {
		C.av_free(unsafe.Pointer(buf)) // Free buffer on failure
		C.av_frame_free(&s.rgbaFrame)
		return fmt.Errorf("%d: couldn't fill the image arrays", status)
	}

	s.swsCtx = C.sws_getContext(s.codecCtx.width,
		s.codecCtx.height, s.codecCtx.pix_fmt,
		C.int(width), C.int(height),
		C.AV_PIX_FMT_RGBA, C.int(alg), nil, nil, nil)
	if s.swsCtx == nil {
		C.av_free(unsafe.Pointer(buf)) // Free buffer
		C.av_frame_free(&s.rgbaFrame)
		return fmt.Errorf("couldn't create an SWS context")
	}

	return nil
}

// ReadFrame reads the next frame from the stream.
func (s *VideoStream) ReadFrame() (Frame, bool, error) {
	return s.ReadVideoFrame()
}

// ReadVideoFrame reads the next video frame from the video stream.
func (s *VideoStream) ReadVideoFrame() (*VideoFrame, bool, error) {
	ok, err := s.read()
	if err != nil {
		return nil, false, err
	}
	if ok && s.skip {
		return nil, true, nil
	}
	if !ok {
		return nil, false, nil
	}

	// Convert frame to RGBA using sws_scale
	C.sws_scale(s.swsCtx, &s.frame.data[0],
		&s.frame.linesize[0], 0,
		s.codecCtx.height,
		&s.rgbaFrame.data[0],
		&s.rgbaFrame.linesize[0])

	// Convert the frame data to Go []byte
	data := C.GoBytes(unsafe.Pointer(s.rgbaFrame.data[0]), s.bufSize)

	frame := newVideoFrame(s, int64(s.frame.pts),
		int(s.frame.coded_picture_number),
		int(s.frame.display_picture_number),
		int(s.codecCtx.width), int(s.codecCtx.height), data)

	return frame, true, nil
}

// Close closes the video stream for decoding.
func (s *VideoStream) Close() error {
	err := s.close()
	if err != nil {
		return err
	}

	if s.rgbaFrame != nil {
		C.av_free(unsafe.Pointer(s.rgbaFrame.data[0]))
		C.av_frame_free(&s.rgbaFrame)
		s.rgbaFrame = nil
	}

	if s.swsCtx != nil {
		C.sws_freeContext(s.swsCtx) // Free SwsContext
		s.swsCtx = nil
	}

	return nil
}
