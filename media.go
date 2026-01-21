package reisen

// #cgo pkg-config: libavformat libavcodec libavutil libswscale libavdevice
// #include <libavcodec/avcodec.h>
// #include <libavformat/avformat.h>
// #include <libavutil/avconfig.h>
// #include <libswscale/swscale.h>
// #include <libavcodec/bsf.h>
// #include <libavdevice/avdevice.h>
import "C"

import (
	"fmt"
	"strconv"
	"time"
	"unsafe"
)

// Media is a media file containing audio, video and other types of streams.
type Media struct {
	ctx     *C.AVFormatContext
	packet  *C.AVPacket
	opts    *Options
	streams []Stream
}

// Options contains the options for the media.
type Options struct {
	// InputFormat short name of the input format.
	InputFormat string

	// Timeout for NewMediaWithOptions when trying to connect to streams.
	Timeout time.Duration
}

// NewMedia returns a new media container for the specified media file.
func NewMedia(filename string) (*Media, error) {
	return NewMediaWithOptions(filename, &Options{})
}

// NewMediaWithOptions returns a new media container for the specified media file
// using the specified options.
func NewMediaWithOptions(filename string, opts *Options) (*Media, error) {
	media := &Media{
		ctx:  C.avformat_alloc_context(),
		opts: opts,
	}

	if media.ctx == nil {
		return nil, fmt.Errorf("couldn't create a new media context")
	}

	var inputFormat *C.AVInputFormat
	var dict *C.AVDictionary
	if opts != nil {
		if opts.InputFormat != "" {
			C.avdevice_register_all()

			cInputFormat := C.CString(opts.InputFormat)
			defer C.free(unsafe.Pointer(cInputFormat))
			inputFormat = C.av_find_input_format(cInputFormat)
			if inputFormat == nil {
				return nil, fmt.Errorf("couldn't find input format %q", opts.InputFormat)
			}
		}
		if opts.Timeout != 0 {
			cTimeout := C.CString(strconv.FormatInt(opts.Timeout.Microseconds(), 10))
			defer C.free(unsafe.Pointer(cTimeout))
			C.av_dict_set(&dict, C.CString("stimeout"), cTimeout, 0) // rtsp
			C.av_dict_set(&dict, C.CString("timeout"), cTimeout, 0)  // tcp/http
		}
	}

	fname := C.CString(filename)
	if C.avformat_open_input(&media.ctx, fname, inputFormat, &dict) < 0 {
		return nil, fmt.Errorf("couldn't open file %s", filename)
	}

	C.free(unsafe.Pointer(fname))
	if err := media.findStreams(); err != nil {
		return nil, err
	}

	return media, nil
}

// StreamCount returns the number of streams.
func (m *Media) StreamCount() int {
	return int(m.ctx.nb_streams)
}

// Streams returns a slice of all the available media data streams.
func (m *Media) Streams() []Stream {
	streams := make([]Stream, len(m.streams))
	copy(streams, m.streams)

	return streams
}

// VideoStreams returns all the video streams of the media file.
func (m *Media) VideoStreams() []*VideoStream {
	videoStreams := []*VideoStream{}

	for _, stream := range m.streams {
		if videoStream, ok := stream.(*VideoStream); ok {
			videoStreams = append(videoStreams, videoStream)
		}
	}

	return videoStreams
}

// AudioStreams returns all the audio streams of the media file.
func (m *Media) AudioStreams() []*AudioStream {
	audioStreams := []*AudioStream{}

	for _, stream := range m.streams {
		if audioStream, ok := stream.(*AudioStream); ok {
			audioStreams = append(audioStreams, audioStream)
		}
	}

	return audioStreams
}

// Duration returns the overall duration
// of the media file.
func (m *Media) Duration() (time.Duration, error) {
	dur := m.ctx.duration
	tm := float64(dur) / float64(TimeBase)

	return time.ParseDuration(fmt.Sprintf("%fs", tm))
}

// FormatName returns the name of the media format.
func (m *Media) FormatName() string {
	if m.ctx.iformat.name == nil {
		return ""
	}

	return C.GoString(m.ctx.iformat.name)
}

// FormatLongName returns the long name
// of the media container.
func (m *Media) FormatLongName() string {
	if m.ctx.iformat.long_name == nil {
		return ""
	}

	return C.GoString(m.ctx.iformat.long_name)
}

// FormatMIMEType returns the MIME type name
// of the media container.
func (media *Media) FormatMIMEType() string {
	if media.ctx.iformat.mime_type == nil {
		return ""
	}

	return C.GoString(media.ctx.iformat.mime_type)
}

// findStreams retrieves the stream information
// from the media container.
func (m *Media) findStreams() error {
	streams := []Stream{}

	if C.avformat_find_stream_info(m.ctx, nil) < 0 {
		return fmt.Errorf("couldn't find stream information")
	}

	innerStreams := unsafe.Slice(m.ctx.streams, m.ctx.nb_streams)

	for _, innerStream := range innerStreams {
		codecParams := innerStream.codecpar
		codec := C.avcodec_find_decoder(codecParams.codec_id)
		if codec == nil {
			unknownStream := new(UnknownStream)
			unknownStream.inner = innerStream
			unknownStream.params = codecParams
			unknownStream.media = m

			streams = append(streams, unknownStream)

			continue
		}

		switch codecParams.codec_type {
		case C.AVMEDIA_TYPE_VIDEO:
			videoStream := new(VideoStream)
			videoStream.inner = innerStream
			videoStream.params = codecParams
			videoStream.codec = codec
			videoStream.media = m

			streams = append(streams, videoStream)

		case C.AVMEDIA_TYPE_AUDIO:
			audioStream := new(AudioStream)
			audioStream.inner = innerStream
			audioStream.params = codecParams
			audioStream.codec = codec
			audioStream.media = m

			streams = append(streams, audioStream)

		default:
			unknownStream := new(UnknownStream)
			unknownStream.inner = innerStream
			unknownStream.params = codecParams
			unknownStream.codec = codec
			unknownStream.media = m

			streams = append(streams, unknownStream)
		}
	}

	m.streams = streams
	return nil
}

// OpenDecode opens the media container for decoding.
//
// CloseDecode() should be called afterwards.
func (m *Media) OpenDecode() error {
	m.packet = C.av_packet_alloc()

	if m.packet == nil {
		return fmt.Errorf(
			"couldn't allocate a new packet")
	}

	return nil
}

// ReadPacket reads the next packet from the media stream.
func (m *Media) ReadPacket() (*Packet, bool, error) {
	if r := C.av_read_frame(m.ctx, m.packet); r < 0 {
		if r == C.int(ErrorAgain) {
			return nil, true, nil
		}

		// No packets anymore.
		return nil, false, nil
	}

	// Filter the packet if needed.
	packetStream := m.streams[m.packet.stream_index]
	outPacket := m.packet

	if packetStream.filter() != nil {
		filter := packetStream.filter()
		packetIn := packetStream.filterIn()
		packetOut := packetStream.filterOut()

		if status := C.av_packet_ref(packetIn, m.packet); status < 0 {
			return nil, false, fmt.Errorf("%d: couldn't reference the packet", status)
		}

		if status := C.av_bsf_send_packet(filter, packetIn); status < 0 {
			return nil, false, fmt.Errorf("%d: couldn't send the packet to the filter", status)
		}

		if status := C.av_bsf_receive_packet(filter, packetOut); status < 0 {
			return nil, false, fmt.Errorf("%d: couldn't receive the packet from the filter", status)
		}

		outPacket = packetOut
	}

	return newPacket(m, outPacket), true, nil
}

// CloseDecode closes the media container for decoding.
func (m *Media) CloseDecode() error {
	C.av_packet_free(&m.packet)
	m.packet = nil

	return nil
}

// Close closes the media container.
func (m *Media) Close() {
	C.avformat_close_input(&m.ctx)
	m.ctx = nil
}
