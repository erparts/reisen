package reisen

// #include <libavformat/avformat.h>
import "C"
import "fmt"

func NetworkInitialize() error {
	if code := C.avformat_network_init(); code < 0 {
		return fmt.Errorf("error occurred: 0x%X", code)
	}

	return nil
}

func NetworkDeinitialize() error {
	if code := C.avformat_network_deinit(); code < 0 {
		return fmt.Errorf("error occurred: 0x%X", code)
	}

	return nil
}
