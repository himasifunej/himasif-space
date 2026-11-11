//go:build windows && !js

package main

import (
	"math"
	"syscall"
	"unsafe"
)

var islandUser32 = syscall.NewLazyDLL("user32.dll")
var islandGDI32 = syscall.NewLazyDLL("gdi32.dll")
var islandQueryState = syscall.NewLazyDLL("shell32.dll").NewProc("SHQueryUserNotificationState")

// The procedures are looked up once: the island asks some at every frame
// of its motion.
var (
	procIslandGetForegroundWindow   = islandUser32.NewProc("GetForegroundWindow")
	procIslandGetDesktopWindow      = islandUser32.NewProc("GetDesktopWindow")
	procIslandGetShellWindow        = islandUser32.NewProc("GetShellWindow")
	procIslandGetWindowRect         = islandUser32.NewProc("GetWindowRect")
	procIslandGetWindowLongPtr      = islandUser32.NewProc("GetWindowLongPtrW")
	procIslandSetWindowLongPtr      = islandUser32.NewProc("SetWindowLongPtrW")
	procIslandMonitorFromWindow     = islandUser32.NewProc("MonitorFromWindow")
	procIslandMonitorFromPoint      = islandUser32.NewProc("MonitorFromPoint")
	procIslandGetMonitorInfo        = islandUser32.NewProc("GetMonitorInfoW")
	procIslandIsIconic              = islandUser32.NewProc("IsIconic")
	procIslandGetCursorPos          = islandUser32.NewProc("GetCursorPos")
	procIslandScreenToClient        = islandUser32.NewProc("ScreenToClient")
	procIslandGetClientRect         = islandUser32.NewProc("GetClientRect")
	procIslandSetWindowPos          = islandUser32.NewProc("SetWindowPos")
	procIslandSetWindowRgn          = islandUser32.NewProc("SetWindowRgn")
	procIslandSetCompositionAttr    = islandUser32.NewProc("SetWindowCompositionAttribute")
	procIslandCreateRoundRectRgn    = islandGDI32.NewProc("CreateRoundRectRgn")
	procIslandDeleteObject          = islandGDI32.NewProc("DeleteObject")
	procIslandCreateEllipticRgn     = islandGDI32.NewProc("CreateEllipticRgn")
	procIslandCombineRgn            = islandGDI32.NewProc("CombineRgn")
	procIslandGetDpiForMonitor      = syscall.NewLazyDLL("shcore.dll").NewProc("GetDpiForMonitor")
	procIslandDwmSetWindowAttribute = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
)

func islandSupported() bool { return true }

// Fail closed when Windows cannot tell us whether interruption is welcome.
// QUNS_ACCEPTS_NOTIFICATIONS=5 excludes busy, D3D fullscreen, presentation,
// quiet time and locked screens. Borderless fullscreen also needs geometry.
func islandEnvironmentAllowed() bool {
	state, ok := islandUserState()
	return ok && state == 5 && !islandForegroundFullscreen()
}

// islandObstructed tells whether the island, already on screen, should
// step aside: a fullscreen app or game (QUNS_BUSY=2, QUNS_RUNNING_D3D_FULL_SCREEN=3,
// QUNS_APP=7), a presentation (4), a locked screen (QUNS_NOT_PRESENT=1),
// or a borderless window over the monitor.
// Quiet time and an unknown state keep it, unlike a new interruption.
func islandObstructed() bool {
	if state, ok := islandUserState(); ok && (state == 1 || state == 2 || state == 3 || state == 4 || state == 7) {
		return true
	}
	return islandForegroundFullscreen()
}

func islandUserState() (uint32, bool) {
	var state uint32
	hr, _, _ := islandQueryState.Call(uintptr(unsafe.Pointer(&state)))
	return state, int32(hr) >= 0
}

func islandForegroundFullscreen() bool {
	hwnd, _, _ := procIslandGetForegroundWindow.Call()
	if hwnd == 0 {
		return false
	}
	// The shell desktop covers the monitor too, but is never a fullscreen app.
	desktop, _, _ := procIslandGetDesktopWindow.Call()
	shell, _, _ := procIslandGetShellWindow.Call()
	if hwnd == desktop || hwnd == shell {
		return false
	}
	// A window with a title bar is not fullscreen, even maximized over a
	// monitor whose taskbar hides itself; borderless video and games have
	// none. Windows reports the other kinds through QUNS above.
	const wsCaption = 0x00C00000
	if style, _, _ := procIslandGetWindowLongPtr.Call(hwnd, uintptr(islandGWLStyle)); style&wsCaption == wsCaption {
		return false
	}
	var r islandRect
	ok, _, _ := procIslandGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	if ok == 0 {
		return false
	}
	monitor, _, _ := procIslandMonitorFromWindow.Call(hwnd, 2)
	info := struct {
		Size          uint32
		Monitor, Work islandRect
		Flags         uint32
	}{Size: 40}
	ok, _, _ = procIslandGetMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return false
	}
	m := info.Monitor
	return r.Left <= m.Left && r.Top <= m.Top && r.Right >= m.Right && r.Bottom >= m.Bottom
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

// islandPointer is where the pointer is over the window, in the DIP of a
// window w×h: physical pixels at the boundary, as islandPosition.
func islandPointer(hwnd uintptr, w, h float64) (float64, float64, bool) {
	var pt struct{ X, Y int32 }
	var rect islandRect
	ok, _, _ := procIslandGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	if ok == 0 {
		return 0, 0, false
	}
	procIslandScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(&pt)))
	ok, _, _ = procIslandGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&rect)))
	if ok == 0 || rect.Right == 0 || rect.Bottom == 0 {
		return 0, 0, false
	}
	return float64(pt.X) * w / float64(rect.Right), float64(pt.Y) * h / float64(rect.Bottom), true
}

func islandPrepareWindow(hwnd uintptr) {
	// WS_EX_NOACTIVATE keeps keyboard focus in the foreground application even
	// on hover/click. Buttons remain mouse-operable; Open explicitly focuses Space.
	style, _, _ := procIslandGetWindowLongPtr.Call(hwnd, uintptr(islandGWLExStyle))
	procIslandSetWindowLongPtr.Call(hwnd, uintptr(islandGWLExStyle), style|0x08000000|0x00000080)
}

// islandWindowShape cuts the window to the pill (g: width, height,
// radius) and the second activity's circle (g[3]) beside it, in a window
// w×h DIP. A native region, rather than global click-through polling,
// removes the transparent corners from Windows hit testing.
func islandWindowShape(hwnd uintptr, g [4]float64, w, h int) {
	var client islandRect
	ok, _, _ := procIslandGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client)))
	if ok == 0 || w == 0 {
		return
	}
	s := float64(client.Right) / float64(w)
	px := func(v float64) uintptr { return uintptr(int(v*s + .5)) }
	radius := px(2 * math.Min(g[2], g[1]/2))
	region, _, _ := procIslandCreateRoundRectRgn.Call(0, 0, px(g[0])+1, px(g[1])+1, radius, radius)
	if region == 0 {
		return
	}
	if g[3] > .5 {
		x := g[0] + 8 // island.BubbleGap
		if dot, _, _ := procIslandCreateEllipticRgn.Call(px(x), 0, px(x+g[3])+1, px(g[3])+1); dot != 0 {
			procIslandCombineRgn.Call(region, region, dot, 2) // RGN_OR
			procIslandDeleteObject.Call(dot)
		}
	}
	ok, _, _ = procIslandSetWindowRgn.Call(hwnd, region, 1)
	if ok == 0 {
		procIslandDeleteObject.Call(region)
	} // OS owns successful regions.
}

// islandClearBackdrop makes what the pill leaves out show the desktop.
// MyGo's native content keeps its alpha in the window's bitmap, which DWM
// composes with it only under an accent; the blur accent of 0.4.4 also
// filled the whole rectangle with its tint (0x99402020, the purple corners
// owners saw), whatever the region. A transparent gradient of color 0 keeps
// the alpha and adds nothing. Windows 11 would also round the tool window
// itself and draw its border around the rectangle: the pill draws both.
func islandClearBackdrop(hwnd uintptr) {
	if procIslandSetCompositionAttr.Find() == nil {
		// ACCENT_ENABLE_TRANSPARENTGRADIENT; flag 2 takes GradientColor as is.
		policy := struct{ State, Flags, Color, Animation uint32 }{State: 2, Flags: 2}
		data := struct {
			Attribute uint32
			Data      uintptr
			Size      uintptr
		}{19, uintptr(unsafe.Pointer(&policy)), unsafe.Sizeof(policy)} // WCA_ACCENT_POLICY
		procIslandSetCompositionAttr.Call(hwnd, uintptr(unsafe.Pointer(&data)))
	}
	if procIslandDwmSetWindowAttribute.Find() == nil {
		corner := uint32(1)          // DWMWCP_DONOTROUND
		border := uint32(0xFFFFFFFE) // DWMWA_COLOR_NONE
		procIslandDwmSetWindowAttribute.Call(hwnd, 33, uintptr(unsafe.Pointer(&corner)), 4)
		procIslandDwmSetWindowAttribute.Call(hwnd, 34, uintptr(unsafe.Pointer(&border)), 4)
	}
}
