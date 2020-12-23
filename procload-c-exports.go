package vorlage

// #include <stdint.h>
// #include <stdio.h>
// #include <stdlib.h>
import "C"
import (
	"./vorlage-interface/golang/vorlageproc"
	"fmt"
	"io"
	"sync"
	"unsafe"
)

const CProcessorsMaxConcurrentStreamInputs = 0x20000

var cDescriptors = make([]vorlageproc.StreamInput, CProcessorsMaxConcurrentStreamInputs)
var descriptorsMutex sync.Mutex

type nilStream int

type nilError int

func (ne nilError) Error() string {
	return ""
}

var nilerror = nilError(0)

func (n2 nilStream) Read(p []byte) (n int, err error) {
	return 0, nilerror
}
func (n2 nilStream) Close() error {
	return nil
}

var nilstream = nilStream(0)

func createCDescriptor(input vorlageproc.StreamInput) *C.int {
	descriptorsMutex.Lock()
	defer descriptorsMutex.Unlock()
	for i := 0; i < len(cDescriptors); i++ {
		if cDescriptors[i] == nil {
			newInt := (*C.int)(C.malloc(C.sizeof_int))
			if input == nil {
				cDescriptors[i] = nilstream
			} else {
				cDescriptors[i] = input
			}
			*newInt = C.int(i)
			return newInt
		}
	}

	// todo: I need to make it so that when the descriptor index becomes full
	//       to allocate more into the index. (block allocation/smart allocation?)
	panic(fmt.Sprintf("vorlageproc buffer for streamed inputs is full, vorlageproc was built to only handle a max amount of %d of concurrent stream inputs (CProcessorsMaxConcurrentStreamInputs). If you get this error, please contact the vorlageproc team for help.", CProcessorsMaxConcurrentStreamInputs))
}
func getCDescriptor(id *C.int) vorlageproc.StreamInput {
	descriptorsMutex.Lock()
	defer descriptorsMutex.Unlock()

	return cDescriptors[int(*id)]
}

func deleteCDescriptor(id *C.int) {
	descriptorsMutex.Lock()
	defer descriptorsMutex.Unlock()

	err := cDescriptors[int(*id)].Close()
	if err != nil {
		Logger.Errorf("vorlageproc failed to close streamed input: %s", err.Error())
	}
	cDescriptors[int(*id)] = nil
	//fmt.Printf("closing %d\n", *id)
	C.free(unsafe.Pointer(id))
}

//export vorlage_stream_read
func vorlage_stream_read(streamptr unsafe.Pointer, buf *C.char, size C.size_t) C.int {
	descriptorId := (*C.int)(streamptr)
	stream := getCDescriptor(descriptorId)
	if stream == nilstream {
		return -3
	}
	array := (*[1 << 28]byte)(unsafe.Pointer(buf))[:size:size]
	n, err := stream.Read(array)
	if err != nil {
		if err == io.EOF {
			if n > 0 {
				// let them do another read, so that way we don't return -2
				// when they actually had more bytes to read.
				return C.int(n)
			}
			return -2
		}
		Logger.Errorf("vorlageproc failed to read from streamed input: %s", err.Error())
		return -1
	}
	return C.int(n)
}

/*
//export vorlage_stream_seek
func vorlage_stream_seek(streamptr unsafe.Pointer, offset C.off_t,  whence C.int) C.int {
	return -1
}
*/
/*
//export vorlage_stream_close
func vorlage_stream_close(streamptr unsafe.Pointer) {
	descriptorId := (*C.int)(streamptr)
	stream := getCDescriptor(descriptorId)
	stream.Close()
}*/
