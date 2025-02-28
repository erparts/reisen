package reisen

// #cgo pkg-config: libavformat libavcodec libavutil libswresample
// #include <libavcodec/avcodec.h>
// #include <libavformat/avformat.h>
// #include <libavutil/avutil.h>
// #include <libswresample/swresample.h>
// AVChannelLayout stereo = AV_CHANNEL_LAYOUT_STEREO;
import "C"
import (
	"fmt"
	"unsafe"
)

const (
	// StandardChannelCount is used for
	// audio conversion while decoding
	// audio frames.
	StandardChannelCount = 2
)

// AudioStream is a stream containing audio frames consisting of audio samples.
type AudioStream struct {
	baseStream
	swrCtx     *C.SwrContext
	buffer     *C.uint8_t
	bufferSize C.int
}

// ChannelCount returns the number of channels (1 for mono, 2 for stereo, etc.).
func (s *AudioStream) ChannelCount() int {
	return int(s.params.channels)
}

// SampleRate returns the sample rate of the audio stream.
func (s *AudioStream) SampleRate() int {
	return int(s.params.sample_rate)
}

// FrameSize returns the number of samples contained in one frame of the audio.
func (s *AudioStream) FrameSize() int {
	return int(s.params.frame_size)
}

// Open opens the audio stream to decode audio frames and samples from it.
func (s *AudioStream) Open() error {
	if err := s.open(); err != nil {
		return err
	}

	C.swr_alloc_set_opts2(&s.swrCtx,
		&C.stereo,
		C.AV_SAMPLE_FMT_S16,
		s.codecCtx.sample_rate,
		&s.codecCtx.ch_layout,
		s.codecCtx.sample_fmt,
		s.codecCtx.sample_rate,
		0,
		nil)

	if s.swrCtx == nil {
		return fmt.Errorf("couldn't allocate an SWR context")
	}

	if r := C.swr_init(s.swrCtx); r < 0 {
		return fmt.Errorf("%d: couldn't initialize the SWR context", r)
	}

	s.buffer = nil

	return nil
}

// ReadFrame reads a new frame from the stream.
func (s *AudioStream) ReadFrame() (Frame, bool, error) {
	return s.ReadAudioFrame()
}

// ReadAudioFrame reads a new audio frame from the stream.
func (s *AudioStream) ReadAudioFrame() (*AudioFrame, bool, error) {
	ok, err := s.read()
	if err != nil {
		return nil, false, err
	}

	if ok && s.skip {
		return nil, true, nil
	}

	// No more data.
	if !ok {
		return nil, false, nil
	}

	maxBufferSize := C.av_samples_get_buffer_size(
		nil, StandardChannelCount,
		s.frame.nb_samples,
		C.AV_SAMPLE_FMT_S16, 1)

	if maxBufferSize < 0 {
		return nil, false, fmt.Errorf("%d: couldn't get the max buffer size", maxBufferSize)
	}

	if maxBufferSize > s.bufferSize {
		C.av_free(unsafe.Pointer(s.buffer))
		s.buffer = nil
	}

	if s.buffer == nil {
		s.buffer = (*C.uint8_t)(unsafe.Pointer(C.av_malloc(bufferSize(maxBufferSize))))
		s.bufferSize = maxBufferSize

		if s.buffer == nil {
			return nil, false, fmt.Errorf(
				"couldn't allocate an AV buffer")
		}
	}

	gotSamples := C.swr_convert(s.swrCtx,
		&s.buffer, s.frame.nb_samples,
		&s.frame.data[0], s.frame.nb_samples)

	if gotSamples < 0 {
		return nil, false, fmt.Errorf("%d: couldn't convert the audio frame", gotSamples)
	}

	data := C.GoBytes(unsafe.Pointer(s.buffer), maxBufferSize)
	frame := newAudioFrame(s,
		int64(s.frame.pts),
		int(s.frame.coded_picture_number),
		int(s.frame.display_picture_number), data)

	return frame, true, nil
}

// Close closes the audio stream and stops decoding audio frames.
func (s *AudioStream) Close() error {
	if err := s.close(); err != nil {
		return err
	}

	C.av_free(unsafe.Pointer(s.buffer))
	s.buffer = nil
	C.swr_free(&s.swrCtx)
	s.swrCtx = nil
	return nil
}
