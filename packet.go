package reisen

// #cgo pkg-config: libavformat libavcodec libavutil libswscale
// #include <libavcodec/avcodec.h>
// #include <libavformat/avformat.h>
// #include <libavutil/avconfig.h>
// #include <libswscale/swscale.h>
import "C"
import "unsafe"

// Packet is a piece of encoded data acquired from the media container.
// It can be either a video frame or an audio frame.
type Packet struct {
	media       *Media
	streamIndex int
	data        []byte
	pts         int64
	dts         int64
	pos         int64
	duration    int64
	size        int
	flags       int
}

// newPacket creates a
// new packet info object.
func newPacket(media *Media, cPkt *C.AVPacket) *Packet {
	pkt := &Packet{
		media:       media,
		streamIndex: int(cPkt.stream_index),
		pts:         int64(cPkt.pts),
		dts:         int64(cPkt.dts),
		pos:         int64(cPkt.pos),
		duration:    int64(cPkt.duration),
		size:        int(cPkt.size),
		flags:       int(cPkt.flags),
	}

	if cPkt.data != nil && cPkt.size > 0 {
		pkt.data = C.GoBytes(unsafe.Pointer(cPkt.data), cPkt.size)
	}

	return pkt
}

// StreamIndex returns the index of the stream the packet belongs to.
func (p *Packet) StreamIndex() int {
	return p.streamIndex
}

// Type returns the type of the packet (video or audio).
func (p *Packet) Type() StreamType {
	return p.media.Streams()[p.streamIndex].Type()
}

// Data returns the data encoded in the packet.
func (p *Packet) Data() []byte {
	return p.data
}

// Returns the size of the packet data.
func (p *Packet) Size() int {
	return p.size
}
