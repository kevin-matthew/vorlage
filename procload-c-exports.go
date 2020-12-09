package vorlage

// #include <stdint.h>
// #include <stdio.h>
// #include <stdlib.h>
import "C"
import (
	"fmt"
	"io"
	"sync"
	"unsafe"
)

const CProcessorsMaxConcurrentStreamInputs = 0x200
var cDescriptors = make([]StreamInput, CProcessorsMaxConcurrentStreamInputs)
var descriptorsMutex sync.Mutex

func createCDescriptor(input StreamInput) *C.int {
	for i := 0; i < len(cDescriptors); i++ {cDescriptors[int(*id)]
		if cDescriptors[i] == nil {
			newInt := (*C.int)(C.malloc(C.sizeof_int))
			cDescriptors[i] = input;
			*newInt = C.int(i)
			return newInt
		}
	}
	// todo: I need to make it so that when the descriptor index becomes full
	//       to allocate more into the index. (block allocation/smart allocation?)
	panic(fmt.Sprintf("vorlage buffer for streamed inputs is full, vorlage was built to only handle a max amount of %d of concurrent stream inputs (CProcessorsMaxConcurrentStreamInputs). If you get this error, please contact the vorlage team for help.", CProcessorsMaxConcurrentStreamInputs))
}
func getCDescriptor(id *C.int) StreamInput {
	return cDescriptors[int(*id)]
}

func deleteCDescriptor(id *C.int) {
	if cDescriptors[int(*id)] == nil {
		return
	}
	err := cDescriptors[int(*id)].Close()
	if err != nil {
		logger.Errorf("vorlage failed to close streamed input: %s", err.Error())
	}
	cDescriptors[int(*id)] = nil
	C.free(unsafe.Pointer(id))
}

//export vorlage_stream_read
func vorlage_stream_read(streamptr unsafe.Pointer, buf *C.char, size C.size_t) C.int {
	descriptorId := (*C.int)(streamptr)
	stream := getCDescriptor(descriptorId)
	if stream == nil {
		return -3
	}
	array  := (*[1 << 28]byte)(unsafe.Pointer(buf))[:size:size]
	n,err  := stream.Read(array)
	if err != nil {
		if err == io.EOF {
			if n > 0 {
				// let them do another read, so that way we don't return -2
				// when they actually had more bytes to read.
				return C.int(n)
			}
			return -2
		}
		logger.Errorf("vorlage failed to read from streamed input: %s", err.Error())
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