package clipboard

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	cfUnicodetext = 13
	cfText        = 1
	gmemMoveable  = 0x0002
)

var (
	cfHTML int

	user32                  = syscall.MustLoadDLL("user32")
	openClipboard           = user32.MustFindProc("OpenClipboard")
	closeClipboard          = user32.MustFindProc("CloseClipboard")
	emptyClipboard          = user32.MustFindProc("EmptyClipboard")
	setClipboardData        = user32.MustFindProc("SetClipboardData")
	registerClipboardFormat = user32.MustFindProc("RegisterClipboardFormatW")

	kernel32     = syscall.NewLazyDLL("kernel32")
	globalAlloc  = kernel32.NewProc("GlobalAlloc")
	globalFree   = kernel32.NewProc("GlobalFree")
	globalLock   = kernel32.NewProc("GlobalLock")
	globalUnlock = kernel32.NewProc("GlobalUnlock")
	lstrcpy      = kernel32.NewProc("lstrcpyW")
)

func init() {
	err := getHTMLFormatCode()
	if err != nil {
		panic(err)
	}
}

func getHTMLFormatCode() error {
	if cfHTML > 0 {
		return nil
	}

	data, err := syscall.UTF16PtrFromString("HTML Format")
	if err != nil {
		return err
	}

	r, _, _ := registerClipboardFormat.Call(uintptr(unsafe.Pointer(data)))
	if r == 0 {
		return errors.New("RegisterClipboardFormat failed")
	}

	cfHTML = int(r)

	return nil
}

// waitOpenClipboard opens the clipboard, waiting for up to a second to do so.
func waitOpenClipboard() error {
	started := time.Now()
	limit := started.Add(time.Second)
	var r uintptr
	var err error
	for time.Now().Before(limit) {
		r, _, err = openClipboard.Call(0)
		if r != 0 {
			return nil
		}
		time.Sleep(time.Millisecond)
	}
	return err
}

func Write(html, text string) error {
	// LockOSThread ensure that the whole method will keep executing on the same thread from begin to end (it actually locks the goroutine thread attribution).
	// Otherwise if the goroutine switch thread during execution (which is a common practice), the OpenClipboard and CloseClipboard will happen on two different threads, and it will result in a clipboard deadlock.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	err := waitOpenClipboard()
	if err != nil {
		return err
	}

	r, _, err := emptyClipboard.Call(0)
	if r == 0 {
		_, _, _ = closeClipboard.Call()
		return err
	}

	unicodeText, err := syscall.UTF16FromString(text)
	if err != nil {
		return err
	}

	err = writeClipboardData(cfUnicodetext, unicodeText)
	if err != nil {
		return fmt.Errorf("error writing unicode text to clipboard: %w", err)
	}

	err = writeClipboardData(cfHTML, []byte(getHTMLData(html)))
	if err != nil {
		return fmt.Errorf("error writing html to clipboard: %w", err)
	}

	closed, _, err := closeClipboard.Call()
	if closed == 0 {
		return err
	}
	return nil
}

func writeClipboardData[T any](cfType int, data []T) error {
	// "If the hMem parameter identifies a memory object, the object must have
	// been allocated using the function with the GMEM_MOVEABLE flag."
	h, _, err := globalAlloc.Call(gmemMoveable, uintptr(len(data)*int(unsafe.Sizeof(data[0]))+2))
	if h == 0 {
		_, _, _ = closeClipboard.Call()
		return err
	}
	defer func() {
		if h != 0 {
			_, _, _ = globalFree.Call(h)
		}
	}()

	l, _, err := globalLock.Call(h)
	if l == 0 {
		_, _, _ = closeClipboard.Call()
		return err
	}

	r, _, err := lstrcpy.Call(l, uintptr(unsafe.Pointer(&data[0])))
	if r == 0 {
		_, _, _ = closeClipboard.Call()
		return err
	}

	r, _, err = globalUnlock.Call(h)
	if r == 0 {
		var errno *syscall.Errno
		if errors.As(err, &errno) {
			_, _, _ = closeClipboard.Call()
			return err
		}
	}

	r, _, err = setClipboardData.Call(uintptr(cfType), h)
	if r == 0 {
		_, _, _ = closeClipboard.Call()
		return err
	}
	h = 0 // suppress deferred cleanup
	return nil
}

func getHTMLData(html string) string {
	var data string

	header := "Version:0.9\r\n" +
		"StartHTML:<<<<<<<1\r\n" +
		"EndHTML:<<<<<<<2\r\n" +
		"StartFragment:<<<<<<<3\r\n" +
		"EndFragment:<<<<<<<4\r\n" +
		"StartSelection:<<<<<<<3\r\n" +
		"EndSelection:<<<<<<<3\r\n"

	data += header
	startHTML := len(data)

	data += "<html><body>\r\n<!--StartFragment-->\r\n"
	fragmentStart := len(data)

	data += html
	fragmentEnd := len(data)

	data += "<!--EndFragment-->\r\n</html></body>"
	endHTML := len(data)

	data += "\r\n"

	// Backpatch offsets
	data = strings.ReplaceAll(data, "<<<<<<<1", fmt.Sprintf("%8d", startHTML))
	data = strings.ReplaceAll(data, "<<<<<<<2", fmt.Sprintf("%8d", endHTML))
	data = strings.ReplaceAll(data, "<<<<<<<3", fmt.Sprintf("%8d", fragmentStart))
	data = strings.ReplaceAll(data, "<<<<<<<4", fmt.Sprintf("%8d", fragmentEnd))

	return data
}
