package reisen

// #cgo pkg-config: libavutil libavformat libavcodec
// #include <libavcodec/avcodec.h>
// #include <libavformat/avformat.h>
// #include <libavutil/avconfig.h>
// #include <libavcodec/bsf.h>
import "C"
import (
	"fmt"
	"time"
	"unsafe"
)

// StreamType is a type of
// a media stream.
type StreamType int

const (
	// StreamVideo denotes the stream keeping video frames.
	StreamVideo StreamType = C.AVMEDIA_TYPE_VIDEO
	// StreamAudio denotes the stream keeping audio frames.
	StreamAudio StreamType = C.AVMEDIA_TYPE_AUDIO
)

// String returns the string representation of
// stream type identifier.
func (streamType StreamType) String() string {
	switch streamType {
	case StreamVideo:
		return "video"

	case StreamAudio:
		return "audio"

	default:
		return ""
	}
}

// TODO: add an opportunity to
// receive duration in time base units.

// Stream is an abstract media data stream.
type Stream interface {
	// innerStream returns the inner
	// libAV stream of the Stream object.
	innerStream() *C.AVStream

	// filter returns the filter context of the stream.
	filter() *C.AVBSFContext
	// filterIn returns the input
	// packet for the stream filter.
	filterIn() *C.AVPacket
	// filterOut returns the output
	// packet for the stream filter.
	filterOut() *C.AVPacket

	// open opens the stream for decoding.
	open() error
	// read decodes the packet and obtains a
	// frame from it.
	read() (bool, error)
	// close closes the stream for decoding.
	close() error

	// Index returns the index
	// number of the stream.
	Index() int
	// Type returns the type
	// identifier of the stream.
	//
	// It's either video or audio.
	Type() StreamType
	// CodecName returns the
	// shortened name of the stream codec.
	CodecName() string
	// CodecLongName returns the
	// long name of the stream codec.
	CodecLongName() string
	// BitRate returns the stream
	// bitrate (in bps).
	BitRate() int64
	// Duration returns the time
	// duration of the stream
	Duration() (time.Duration, error)
	// TimeBase returns the numerator
	// and the denominator of the stream
	// time base fraction to convert
	// time duration in time base units
	// of the stream.
	TimeBase() (int, int)
	// FrameRate returns the approximate
	// frame rate (FPS) of the stream.
	FrameRate() (int, int)
	// FrameCount returns the total number
	// of frames in the stream.
	FrameCount() int64
	// Open opens the stream for decoding.
	Open() error
	// Rewind rewinds the whole media to the
	// specified time location based on the stream.
	Rewind(time.Duration) error
	// ApplyFilter applies a filter defined
	// by the given string to the stream.
	ApplyFilter(string) error
	// Filter returns the name and arguments
	// of the filter currently applied to the
	// stream or "" if no filter applied.
	Filter() string
	// RemoveFilter removes the currently applied
	// filter from the stream and frees its memory.
	RemoveFilter() error
	// ReadFrame decodes the next frame from the stream.
	ReadFrame() (Frame, bool, error)
	// Closes the stream for decoding.
	Close() error
}

// baseStream holds the information common for all media data streams.
type baseStream struct {
	media           *Media
	inner           *C.AVStream
	params          *C.AVCodecParameters
	codec           *C.AVCodec
	codecCtx        *C.AVCodecContext
	frame           *C.AVFrame
	filterArgs      string
	filterCtx       *C.AVBSFContext
	filterInPacket  *C.AVPacket
	filterOutPacket *C.AVPacket
	skip            bool
	opened          bool
}

// Opened returns 'true' if the stream is opened for decoding, and 'false' otherwise.
func (s *baseStream) Opened() bool {
	return s.opened
}

// Index returns the index of the stream.
func (s *baseStream) Index() int {
	return int(s.inner.index)
}

// Type returns the stream media data type.
func (s *baseStream) Type() StreamType {
	return StreamType(s.params.codec_type)
}

// CodecName returns the name of the codec that was used for encoding the stream.
func (s *baseStream) CodecName() string {
	if s.codec.name == nil {
		return ""
	}

	return C.GoString(s.codec.name)
}

// CodecName returns the long name of the codec that was used for encoding the stream.
func (s *baseStream) CodecLongName() string {
	if s.codec.long_name == nil {
		return ""
	}

	return C.GoString(s.codec.long_name)
}

// BitRate returns the bit rate of the stream (in bps).
func (s *baseStream) BitRate() int64 {
	return int64(s.params.bit_rate)
}

// Duration returns the duration of the stream.
func (s *baseStream) Duration() (time.Duration, error) {
	dur := s.inner.duration

	if dur < 0 {
		dur = 0
	}

	tmNum, tmDen := s.TimeBase()
	factor := float64(tmNum) / float64(tmDen)
	tm := float64(dur) * factor

	return time.ParseDuration(fmt.Sprintf("%fs", tm))
}

// TimeBase the numerator and the denominator of the stream time base factor fraction.
//
// All the duration values of the stream are multiplied by this factor to get duration
// in seconds.
func (s *baseStream) TimeBase() (int, int) {
	return int(s.inner.time_base.num),
		int(s.inner.time_base.den)
}

// FrameRate returns the frame rate of the stream as a fraction with a numerator and a denominator.
func (s *baseStream) FrameRate() (int, int) {
	return int(s.inner.r_frame_rate.num),
		int(s.inner.r_frame_rate.den)
}

// FrameCount returns the total number of frames in the stream.
func (s *baseStream) FrameCount() int64 {
	return int64(s.inner.nb_frames)
}

// ApplyFilter applies a filter defined by the given string to the stream.
func (s *baseStream) ApplyFilter(args string) error {
	cArgs := C.CString(args)
	defer C.free(unsafe.Pointer(cArgs))

	if r := C.av_bsf_list_parse_str(cArgs, &s.filterCtx); r < 0 {
		return fmt.Errorf(
			"%d: couldn't create a filter context", r)
	}

	if r := C.avcodec_parameters_copy(s.filterCtx.par_in, s.params); r < 0 {
		return fmt.Errorf("%d: couldn't copy the input codec parameters to the filter", r)
	}

	if r := C.avcodec_parameters_copy(s.filterCtx.par_out, s.params); r < 0 {
		return fmt.Errorf("%d: couldn't copy the output codec parameters to the filter", r)
	}

	s.filterCtx.time_base_in = s.inner.time_base
	s.filterCtx.time_base_out = s.inner.time_base

	if r := C.av_bsf_init(s.filterCtx); r < 0 {
		return fmt.Errorf("%d: couldn't initialize the filter context", r)
	}

	s.filterInPacket = C.av_packet_alloc()
	if s.filterInPacket == nil {
		return fmt.Errorf("couldn't allocate a packet for filtering in")
	}

	s.filterOutPacket = C.av_packet_alloc()
	if s.filterInPacket == nil {
		return fmt.Errorf("couldn't allocate a packet for filtering out")
	}

	s.filterArgs = args

	return nil
}

// Filter returns the name and arguments of the filter currently applied to the
// stream or "" if no filter applied.
func (s *baseStream) Filter() string {
	return s.filterArgs
}

// RemoveFilter removes the currently applied filter from the stream and frees its memory.
func (s *baseStream) RemoveFilter() error {
	if s.filterCtx == nil {
		return fmt.Errorf("no filter applied")
	}

	C.av_bsf_free(&s.filterCtx)
	s.filterCtx = nil

	C.av_packet_free(&s.filterInPacket)
	C.av_packet_free(&s.filterOutPacket)
	s.filterInPacket = nil
	s.filterOutPacket = nil

	return nil
}

// Rewind rewinds the stream to the specified time position.
//
// Can be used on all the types of streams. However, it's better to use it on
// the video stream ofthe media file if you don't want the streams of the
// playback to desynchronyze.
func (s *baseStream) Rewind(t time.Duration) error {
	tmNum, tmDen := s.TimeBase()
	factor := float64(tmDen) / float64(tmNum)
	seconds := t.Seconds()
	dur := int64(seconds * factor)

	r := C.av_seek_frame(s.media.ctx,
		s.inner.index, rewindPosition(dur),
		C.AVSEEK_FLAG_FRAME|C.AVSEEK_FLAG_BACKWARD)

	if r < 0 {
		return fmt.Errorf(
			"%d: couldn't rewind the stream", r)
	}

	return nil
}

// innerStream returns the inner
// libAV stream of the Stream object.
func (s *baseStream) innerStream() *C.AVStream {
	return s.inner
}

// filter returns the filter context of the stream.
func (s *baseStream) filter() *C.AVBSFContext {
	return s.filterCtx
}

// filterIn returns the input
// packet for the stream filter.
func (s *baseStream) filterIn() *C.AVPacket {
	return s.filterInPacket
}

// filterOut returns the output
// packet for the stream filter.
func (s *baseStream) filterOut() *C.AVPacket {
	return s.filterOutPacket
}

// open opens the stream for decoding.
func (s *baseStream) open() error {
	s.codecCtx = C.avcodec_alloc_context3(s.codec)
	if s.codecCtx == nil {
		return fmt.Errorf("couldn't open a codec context")
	}

	if r := C.avcodec_parameters_to_context(s.codecCtx, s.params); r < 0 {
		return fmt.Errorf("%d: couldn't send codec parameters to the context", r)
	}

	if r := C.avcodec_open2(s.codecCtx, s.codec, nil); r < 0 {
		return fmt.Errorf("%d: couldn't open the codec context", r)
	}

	s.frame = C.av_frame_alloc()
	if s.frame == nil {
		return fmt.Errorf("couldn't allocate a new frame")
	}

	s.opened = true
	return nil
}

// read decodes the packet and obtains a frame from it.
func (s *baseStream) read() (bool, error) {
	readPacket := s.media.packet

	if s.filterCtx != nil {
		readPacket = s.filterOutPacket
	}

	if r := C.avcodec_send_packet(s.codecCtx, readPacket); r < 0 {
		s.skip = false
		return false, fmt.Errorf("%d: couldn't send the packet to the codec context", r)
	}

	if r := C.avcodec_receive_frame(s.codecCtx, s.frame); r < 0 {
		if r == C.int(ErrorAgain) {
			s.skip = true
			return true, nil
		}

		s.skip = false
		return false, fmt.Errorf("%d: couldn't receive the frame from the codec context", r)
	}

	C.av_packet_unref(s.media.packet)

	if s.filterInPacket != nil {
		C.av_packet_unref(s.filterInPacket)
	}

	if s.filterOutPacket != nil {
		C.av_packet_unref(s.filterOutPacket)
	}

	s.skip = false

	return true, nil
}

// close closes the stream for decoding.
func (s *baseStream) close() error {
	C.av_frame_free(&s.frame)
	s.frame = nil

	C.avcodec_free_context(&s.codecCtx)
	s.codecCtx = nil

	if s.filterCtx != nil {
		C.av_bsf_free(&s.filterCtx)
		s.filterCtx = nil
	}

	if s.filterInPacket != nil {
		C.av_free(unsafe.Pointer(s.filterInPacket))
		s.filterInPacket = nil
	}

	if s.filterOutPacket != nil {
		C.av_free(unsafe.Pointer(s.filterOutPacket))
		s.filterOutPacket = nil
	}

	s.opened = false

	return nil
}
