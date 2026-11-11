//go:build windows && !js

package main

import (
	"runtime"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"
)

// The current media session through WinRT's
// Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager
// (Windows 10 1809 and later), without cgo: COM calls by vtable index.
// Interface IDs and method order are Windows SDK's windows.media.control
// metadata, as windows-rs generates it
// (crates/libs/windows/src/Windows/Media/Control/mod.rs); each index
// counts IInspectable's six methods first.

var (
	islandCombase                 = syscall.NewLazyDLL("combase.dll")
	procIslandRoInitialize        = islandCombase.NewProc("RoInitialize")
	procIslandRoUninitialize      = islandCombase.NewProc("RoUninitialize")
	procIslandRoGetFactory        = islandCombase.NewProc("RoGetActivationFactory")
	procIslandCreateString        = islandCombase.NewProc("WindowsCreateString")
	procIslandDeleteString        = islandCombase.NewProc("WindowsDeleteString")
	procIslandGetStringRawBuffer  = islandCombase.NewProc("WindowsGetStringRawBuffer")
	islandMediaManagerClass       = "Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager"
	iidIslandMediaManagerStatics  = islandGUID{0x2050c4ee, 0x11a0, 0x57de, [8]byte{0xae, 0xd7, 0xc9, 0x7c, 0x70, 0x33, 0x82, 0x45}}
	iidIslandAsyncInfo            = islandGUID{0x00000036, 0x0000, 0x0000, [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	islandMediaOut                = new(islandMediaScratch)
	islandMediaRuntimeInitialized bool
)

// Vtable indices.
const (
	vtRelease             = 2
	vtRequestAsync        = 6 // IGlobalSystemMediaTransportControlsSessionManagerStatics
	vtGetCurrentSession   = 6 // IGlobalSystemMediaTransportControlsSessionManager
	vtSourceAppUserModel  = 6 // IGlobalSystemMediaTransportControlsSession
	vtTryGetProperties    = 7
	vtGetTimeline         = 8
	vtGetPlaybackInfo     = 9
	vtTrySkipNext         = 16
	vtTrySkipPrevious     = 17
	vtTryTogglePlayPause  = 20
	vtPropTitle           = 6 // IGlobalSystemMediaTransportControlsSessionMediaProperties
	vtPropArtist          = 9
	vtPlaybackStatus      = 7  // IGlobalSystemMediaTransportControlsSessionPlaybackInfo
	vtTimelineStart       = 6  // IGlobalSystemMediaTransportControlsSessionTimelineProperties
	vtTimelineEnd         = 7  //
	vtTimelinePosition    = 10 //
	vtTimelineLastUpdated = 11 //
	vtAsyncStatus         = 7  // IAsyncInfo
	vtAsyncGetResults     = 8  // IAsyncOperation<T>
	mediaStatusPlaying    = 4  // GlobalSystemMediaTransportControlsSessionPlaybackStatus
)

type islandGUID struct {
	Data1        uint32
	Data2, Data3 uint16
	Data4        [8]byte
}

// islandMediaScratch holds the calls' out parameters. It is on the heap,
// where Go never moves it, as it hands Windows their addresses as
// integers; only the watcher's goroutine uses it.
type islandMediaScratch struct {
	ptr, result uintptr
	status      int32
	ticks       int64
	length      uint32
}

func islandMediaSupported() bool { return procIslandRoGetFactory.Find() == nil }

func islandNative(addr uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&addr)) }

func islandCOM(obj uintptr, i int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(islandNative(obj))
	fn := *(*uintptr)(unsafe.Add(islandNative(vtbl), i*int(unsafe.Sizeof(uintptr(0)))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{obj}, args...)...)
	return r
}

func islandFailed(hr uintptr) bool { return int32(uint32(hr)) < 0 }

func islandRelease(obj uintptr) {
	if obj != 0 {
		islandCOM(obj, vtRelease)
	}
}

type islandMediaSession struct {
	manager uintptr
	tried   time.Time
}

// newIslandMediaSession binds the watcher's goroutine to its thread, in
// WinRT's multithreaded apartment, where the manager and its sessions live.
func newIslandMediaSession() *islandMediaSession {
	runtime.LockOSThread()
	hr, _, _ := procIslandRoInitialize.Call(1) // RO_INIT_MULTITHREADED
	islandMediaRuntimeInitialized = !islandFailed(hr)
	return &islandMediaSession{}
}

func (s *islandMediaSession) close() {
	islandRelease(s.manager)
	s.manager = 0
	if islandMediaRuntimeInitialized {
		procIslandRoUninitialize.Call()
	}
	runtime.UnlockOSThread()
}

// ensureManager asks Windows for the manager, again at most once a minute
// when it refuses (an old Windows, the API off).
func (s *islandMediaSession) ensureManager() bool {
	if s.manager != 0 {
		return true
	}
	if !s.tried.IsZero() && time.Since(s.tried) < time.Minute {
		return false
	}
	s.tried = time.Now()
	class, err := syscall.UTF16FromString(islandMediaManagerClass)
	if err != nil {
		return false
	}
	o := islandMediaOut
	hr, _, _ := procIslandCreateString.Call(uintptr(unsafe.Pointer(&class[0])), uintptr(len(class)-1), uintptr(unsafe.Pointer(&o.ptr)))
	if islandFailed(hr) {
		return false
	}
	name := o.ptr
	defer procIslandDeleteString.Call(name)
	o.ptr = 0
	hr, _, _ = procIslandRoGetFactory.Call(name, uintptr(unsafe.Pointer(&iidIslandMediaManagerStatics)), uintptr(unsafe.Pointer(&o.ptr)))
	if islandFailed(hr) || o.ptr == 0 {
		return false
	}
	statics := o.ptr
	defer islandRelease(statics)
	o.ptr = 0
	if islandFailed(islandCOM(statics, vtRequestAsync, uintptr(unsafe.Pointer(&o.ptr)))) || o.ptr == 0 {
		return false
	}
	s.manager = islandAwait(o.ptr)
	return s.manager != 0
}

// islandAwait waits for an IAsyncOperation, up to 2 s, and returns its
// result (a reference to release); it releases the operation.
func islandAwait(op uintptr) uintptr {
	defer islandRelease(op)
	o := islandMediaOut
	o.ptr = 0
	hr, _, _ := syscall.SyscallN(*(*uintptr)(islandNative(*(*uintptr)(islandNative(op)))), op, uintptr(unsafe.Pointer(&iidIslandAsyncInfo)), uintptr(unsafe.Pointer(&o.ptr)))
	if islandFailed(hr) || o.ptr == 0 {
		return 0
	}
	info := o.ptr
	defer islandRelease(info)
	for deadline := time.Now().Add(2 * time.Second); ; time.Sleep(5 * time.Millisecond) {
		o.status = 0
		if islandFailed(islandCOM(info, vtAsyncStatus, uintptr(unsafe.Pointer(&o.status)))) {
			return 0
		}
		if o.status == 1 { // Completed
			break
		}
		if o.status != 0 || time.Now().After(deadline) { // Canceled, Error
			return 0
		}
	}
	o.result = 0
	if islandFailed(islandCOM(op, vtAsyncGetResults, uintptr(unsafe.Pointer(&o.result)))) {
		return 0
	}
	return o.result
}

// islandHString reads a getter returning an HSTRING, and frees it.
func islandHString(obj uintptr, i int) string {
	o := islandMediaOut
	o.ptr = 0
	if islandFailed(islandCOM(obj, i, uintptr(unsafe.Pointer(&o.ptr)))) || o.ptr == 0 {
		return ""
	}
	h := o.ptr
	defer procIslandDeleteString.Call(h)
	o.length = 0
	buf, _, _ := procIslandGetStringRawBuffer.Call(h, uintptr(unsafe.Pointer(&o.length)))
	if buf == 0 || o.length == 0 {
		return ""
	}
	return string(utf16.Decode(unsafe.Slice((*uint16)(islandNative(buf)), o.length)))
}

func islandTicks(obj uintptr, i int) (int64, bool) {
	o := islandMediaOut
	o.ticks = 0
	if islandFailed(islandCOM(obj, i, uintptr(unsafe.Pointer(&o.ticks)))) {
		return 0, false
	}
	return o.ticks, true
}

// current is the current session, a reference to release, or 0.
func (s *islandMediaSession) current() uintptr {
	if !s.ensureManager() {
		return 0
	}
	o := islandMediaOut
	o.ptr = 0
	if islandFailed(islandCOM(s.manager, vtGetCurrentSession, uintptr(unsafe.Pointer(&o.ptr)))) {
		// The manager went away with its service: ask again next time.
		islandRelease(s.manager)
		s.manager, s.tried = 0, time.Time{}
		return 0
	}
	return o.ptr
}

func (s *islandMediaSession) read() (islandMedia, bool) {
	session := s.current()
	if session == 0 {
		return islandMedia{}, false
	}
	defer islandRelease(session)
	m := islandMedia{App: islandMediaApp(islandHString(session, vtSourceAppUserModel)), At: time.Now()}
	o := islandMediaOut
	o.ptr = 0
	if !islandFailed(islandCOM(session, vtTryGetProperties, uintptr(unsafe.Pointer(&o.ptr)))) && o.ptr != 0 {
		if props := islandAwait(o.ptr); props != 0 {
			m.Title = islandHString(props, vtPropTitle)
			m.Artist = islandHString(props, vtPropArtist)
			islandRelease(props)
		}
	}
	o.ptr = 0
	if !islandFailed(islandCOM(session, vtGetPlaybackInfo, uintptr(unsafe.Pointer(&o.ptr)))) && o.ptr != 0 {
		info := o.ptr
		o.status = 0
		if !islandFailed(islandCOM(info, vtPlaybackStatus, uintptr(unsafe.Pointer(&o.status)))) {
			m.Playing = o.status == mediaStatusPlaying
		}
		islandRelease(info)
	}
	o.ptr = 0
	if !islandFailed(islandCOM(session, vtGetTimeline, uintptr(unsafe.Pointer(&o.ptr)))) && o.ptr != 0 {
		tl := o.ptr
		start, ok1 := islandTicks(tl, vtTimelineStart)
		end, ok2 := islandTicks(tl, vtTimelineEnd)
		pos, ok3 := islandTicks(tl, vtTimelinePosition)
		updated, ok4 := islandTicks(tl, vtTimelineLastUpdated)
		islandRelease(tl)
		if ok1 && ok2 && ok3 && end > start {
			m.Duration = time.Duration(end-start) * 100
			m.Position = time.Duration(pos-start) * 100
			// DateTime counts 100 ns from 1601; players update it now and then.
			if ok4 && updated > 0 {
				if at := time.Unix(0, (updated-116444736000000000)*100); at.Before(m.At) && m.At.Sub(at) < time.Hour {
					m.At = at
				}
			}
		}
	}
	return m, m.Title != ""
}

func (s *islandMediaSession) command(cmd string) {
	session := s.current()
	if session == 0 {
		return
	}
	defer islandRelease(session)
	i := vtTryTogglePlayPause
	switch cmd {
	case mediaNext:
		i = vtTrySkipNext
	case mediaPrevious:
		i = vtTrySkipPrevious
	}
	o := islandMediaOut
	o.ptr = 0
	if !islandFailed(islandCOM(session, i, uintptr(unsafe.Pointer(&o.ptr)))) && o.ptr != 0 {
		// IAsyncOperation<bool>: the result is not a reference.
		op := o.ptr
		islandAwaitDone(op)
	}
}

// islandAwaitDone waits for an operation whose result is a value, then
// releases it.
func islandAwaitDone(op uintptr) {
	defer islandRelease(op)
	o := islandMediaOut
	o.ptr = 0
	hr, _, _ := syscall.SyscallN(*(*uintptr)(islandNative(*(*uintptr)(islandNative(op)))), op, uintptr(unsafe.Pointer(&iidIslandAsyncInfo)), uintptr(unsafe.Pointer(&o.ptr)))
	if islandFailed(hr) || o.ptr == 0 {
		return
	}
	info := o.ptr
	defer islandRelease(info)
	for deadline := time.Now().Add(time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		o.status = 0
		if islandFailed(islandCOM(info, vtAsyncStatus, uintptr(unsafe.Pointer(&o.status)))) || o.status != 0 {
			return
		}
	}
}
