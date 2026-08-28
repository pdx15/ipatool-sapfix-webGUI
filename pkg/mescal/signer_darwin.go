//go:build darwin && cgo

package mescal

/*
#cgo CFLAGS: -fblocks
#cgo LDFLAGS: -framework Foundation

#include <stddef.h>
#include <stdlib.h>

int ipatool_mescal_sign(
	const unsigned char *input,
	size_t input_length,
	unsigned char **output,
	size_t *output_length,
	char **error_message
);
*/
import "C"

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"
)

var signingMutex sync.Mutex

// Available reports whether this platform can create the binary SAP
// signature Apple expects for protected Store actions.
func Available() bool {
	return true
}

// Sign creates the binary SAP signature Apple expects for protected Store
// actions by using the signing service built into macOS.
func Sign(data []byte) ([]byte, error) {
	signingMutex.Lock()
	defer signingMutex.Unlock()

	var input *C.uchar
	if len(data) > 0 {
		input = (*C.uchar)(unsafe.Pointer(&data[0]))
	}

	var output *C.uchar
	var outputLength C.size_t
	var errorMessage *C.char

	status := C.ipatool_mescal_sign(
		input,
		C.size_t(len(data)),
		&output,
		&outputLength,
		&errorMessage,
	)

	if output != nil {
		defer C.free(unsafe.Pointer(output))
	}

	if errorMessage != nil {
		defer C.free(unsafe.Pointer(errorMessage))
	}

	if status != 0 {
		return nil, signFailureError(status, errorMessage)
	}

	if output == nil || outputLength == 0 {
		return nil, errors.New("CommerceKit returned an empty SAP signature")
	}

	if uint64(outputLength) > uint64(^uint32(0)>>1) {
		return nil, errors.New("CommerceKit returned an oversized SAP signature")
	}

	return C.GoBytes(unsafe.Pointer(output), C.int(outputLength)), nil
}

// signFailureError converts a non-zero CommerceKit signing status into an
// error, exposing the service message when one is available.
func signFailureError(status C.int, errorMessage *C.char) error {
	message := "unknown CommerceKit error"
	if errorMessage != nil {
		message = C.GoString(errorMessage)
	}

	if status == 1 {
		return fmt.Errorf("%w: %s", ErrUnavailable, message)
	}

	return errors.New(message)
}
