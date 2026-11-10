//go:build windows && !js

package main

import (
	"syscall"
	"unsafe"
)

var islandUser32 = syscall.NewLazyDLL("user32.dll")
var islandGDI32 = syscall.NewLazyDLL("gdi32.dll")
var islandQueryState = syscall.NewLazyDLL("shell32.dll").NewProc("SHQueryUserNotificationState")

// The procedures are looked up once: the island asks some at every frame
// of its motion.
var (
	procIslandGetForegroundWindow = islandUser32.NewProc("GetForegroundWindow")
	procIslandGetDesktopWindow    = islandUser32.NewProc("GetDesktopWindow")
	procIslandGetShellWindow      = islandUser32.NewProc("GetShellWindow")
	procIslandGetWindowRect       = islandUser32.NewProc("GetWindowRect")
	procIslandGetWindowLongPtr    = islandUser32.NewProc("GetWindowLongPtrW")
	procIslandSetWindowLongPtr    = islandUser32.NewProc("SetWindowLongPtrW")
	procIslandMonitorFromWindow   = islandUser32.NewProc("MonitorFromWindow")
	procIslandMonitorFromPoint    = islandUser32.NewProc("MonitorFromPoint")
	procIslandGetMonitorInfo      = islandUser32.NewProc("GetMonitorInfoW")
	procIslandIsIconic            = islandUser32.NewProc("IsIconic")
	procIslandGetCursorPos        = islandUser32.NewProc("GetCursorPos")
	procIslandScreenToClient      = islandUser32.NewProc("ScreenToClient")
	procIslandGetClientRect       = islandUser32.NewProc("GetClientRect")
	procIslandSetWindowPos        = islandUser32.NewProc("SetWindowPos")
	procIslandSetWindowRgn        = islandUser32.NewProc("SetWindowRgn")
	procIslandSetCompositionAttr  = islandUser32.NewProc("SetWindowCompositionAttribute")
	procIslandCreateRoundRectRgn  = islandGDI32.NewProc("CreateRoundRectRgn")
	procIslandDeleteObject        = islandGDI32.NewProc("DeleteObject")
	procIslandGetDpiForMonitor    = syscall.NewLazyDLL("shcore.dll").NewProc("GetDpiForMonitor")
)

func islandSupported() bool { return true }

// Fail closed when Windows cannot tell us whether interruption is welcome.
// QUNS_ACCEPTS_NOTIFICATIONS=5 excludes busy, D3D fullscreen, presentation,
// quiet time and locked screens. Borderless fullscreen also needs geometry.
func islandEnvironmentAllowed() bool {
	var state uint32
	hr, _, _ := islandQueryState.Call(uintptr(unsafe.Pointer(&state)))
	if int32(hr) < 0 || state != 5 {
		return false
	}
	hwnd, _, _ := procIslandGetForegroundWindow.Call()
	if hwnd == 0 {
		return true
	}
	// The shell desktop covers the monitor too, but is never a fullscreen app.
	desktop, _, _ := procIslandGetDesktopWindow.Call()
	shell, _, _ := procIslandGetShellWindow.Call()
	if hwnd == desktop || hwnd == shell {
		return true
	}
	// A window with a title bar is not fullscreen, even maximized over a
	// monitor whose taskbar hides itself; borderless video and games have
	// none. Windows reports the other kinds through QUNS above.
	const wsCaption = 0x00C00000
	if style, _, _ := procIslandGetWindowLongPtr.Call(hwnd, uintptr(islandGWLStyle)); style&wsCaption == wsCaption {
		return true
	}
	var r islandRect
	ok, _, _ := procIslandGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	if ok == 0 {
		return true
	}
	monitor, _, _ := procIslandMonitorFromWindow.Call(hwnd, 2)
	info := struct {
		Size          uint32
		Monitor, Work islandRect
		Flags         uint32
	}{Size: 40}
	ok, _, _ = procIslandGetMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return true
	}
	m := info.Monitor
	return !(r.Left <= m.Left && r.Top <= m.Top && r.Right >= m.Right && r.Bottom >= m.Bottom)
}

type islandRect struct{ Left, Top, Right, Bottom int32 }

// GWL_STYLE and GWL_EXSTYLE.
var islandGWLStyle, islandGWLExStyle = int32(-16), int32(-20)

// Use physical Win32 monitor coordinates at the boundary. MyGo's public DIP
// coordinates are per-monitor and can overlap when DPI differs across screens.
func islandPosition(hwnd, space uintptr, cursor bool, w, h int) bool {
	iconic, _, _ := procIslandIsIconic.Call(space)
	var monitor uintptr
	if cursor || iconic != 0 {
		var pt struct{ X, Y int32 }
		procIslandGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
		// MonitorFromPoint receives POINT by value on 64-bit Windows.
		packed := uint64(uint32(pt.X)) | uint64(uint32(pt.Y))<<32
		monitor, _, _ = procIslandMonitorFromPoint.Call(uintptr(packed), 2)
	} else {
		monitor, _, _ = procIslandMonitorFromWindow.Call(space, 2)
	}
	info := struct {
		Size          uint32
		Monitor, Work islandRect
		Flags         uint32
	}{Size: 40}
	ok, _, _ := procIslandGetMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return false
	}
	dpiX, dpiY := uint32(96), uint32(96)
	if procIslandGetDpiForMonitor.Find() == nil {
		procIslandGetDpiForMonitor.Call(monitor, 0, uintptr(unsafe.Pointer(&dpiX)), uintptr(unsafe.Pointer(&dpiY)))
	}
	scale := float64(dpiX) / 96
	pw, ph := int(float64(w)*scale+.5), int(float64(h)*scale+.5)
	x := int(info.Work.Left) + (int(info.Work.Right-info.Work.Left)-pw)/2
	y := int(info.Work.Top) + int(islandTopGap*scale+.5)
	// Moving to a monitor of another DPI makes Windows send WM_DPICHANGED
	// during the move, and MyGo applies the rectangle it suggests, scaled
	// once more. A second move, now in the new DPI, puts it right.
	for range 2 {
		ok, _, _ = procIslandSetWindowPos.Call(hwnd, 0, uintptr(x), uintptr(y), uintptr(pw), uintptr(ph), 0x0014) // NOZORDER | NOACTIVATE
		if ok == 0 {
			return false
		}
		var r islandRect
		if got, _, _ := procIslandGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r))); got == 0 || int(r.Left) == x && int(r.Top) == y && int(r.Right-r.Left) == pw && int(r.Bottom-r.Top) == ph {
			break
		}
	}
	return true
}

func islandPointer(hwnd uintptr, g [3]float64) (bool, bool) {
	var pt struct{ X, Y int32 }
	var rect islandRect
	ok, _, _ := procIslandGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if ok == 0 {
		return false, false
	}
	procIslandScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
	ok, _, _ = procIslandGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	if ok == 0 || rect.Right == 0 || rect.Bottom == 0 {
		return false, false
	}
	return insideIsland(float64(pt.X)*g[0]/float64(rect.Right), float64(pt.Y)*g[1]/float64(rect.Bottom), g), true
}

func islandPrepareWindow(hwnd uintptr) {
	// WS_EX_NOACTIVATE keeps keyboard focus in the foreground application even
	// on hover/click. Buttons remain mouse-operable; Open explicitly focuses Space.
	style, _, _ := procIslandGetWindowLongPtr.Call(hwnd, uintptr(islandGWLExStyle))
	procIslandSetWindowLongPtr.Call(hwnd, uintptr(islandGWLExStyle), style|0x08000000|0x00000080)
}

func islandWindowShape(hwnd uintptr, w, h int, r float64) {
	// A native region, rather than global click-through polling, removes the
	// transparent rounded corners from Windows hit testing at every frame.
	var client islandRect
	ok, _, _ := procIslandGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
	if ok == 0 || w == 0 {
		return
	}
	radius := int(r * float64(client.Right) / float64(w) * 2)
	region, _, _ := procIslandCreateRoundRectRgn.Call(0, 0, uintptr(client.Right+1), uintptr(client.Bottom+1), uintptr(radius), uintptr(radius))
	if region == 0 {
		return
	}
	ok, _, _ = procIslandSetWindowRgn.Call(hwnd, region, 1)
	if ok == 0 {
		procIslandDeleteObject.Call(region)
	} // OS owns successful regions.
}

func islandGlass(hwnd uintptr, on, dark bool) {
	// Windows composition accent blur is optional. A translucent, high-contrast
	// tinted surface remains usable on systems without composition support.
	proc := procIslandSetCompositionAttr
	if proc.Find() != nil {
		return
	}
	policy := struct{ State, Flags, Color, Animation uint32 }{}
	if on {
		policy.State = 3
		policy.Color = 0x99402020
		if !dark {
			policy.Color = 0x99f6f9fa
		}
	}
	data := struct {
		Attribute uint32
		Data      uintptr
		Size      uintptr
	}{19, uintptr(unsafe.Pointer(&policy)), unsafe.Sizeof(policy)}
	proc.Call(hwnd, uintptr(unsafe.Pointer(&data)))
}
