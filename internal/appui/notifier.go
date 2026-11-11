package appui

import "github.com/himasifunej/himasif-space/internal/id"

// The device's notification layer, shared by onboarding, the settings page
// and, in time, deliveries. The web app shows browser popups (the
// Notification API), the desktop app the operating system's
// notifications. A later presenter, such as a "dynamic island" for Space
// desktop, is one more NoticePresenter beside the OS one, put together with
// MultiNotifier. Nothing here needs to change for it.

// Kinds of notice.
const (
	// NoticeTest proves that notifications reach the user, right after
	// they allowed them.
	NoticeTest = "test"
	// NoticeInfo is a short message of Space itself.
	NoticeInfo = "info"
	// NoticeDelivery is a reminder or a ping.
	NoticeDelivery = "delivery"
)

// Notice is one notification to show.
type Notice struct {
	Kind        string
	Title, Body string
	// Tag replaces an earlier notice with the same tag, where the platform
	// can do that.
	Tag string
	// Link is a Space link opened on a click; "" brings Space forward.
	Link string
	// Actions are optional Space deep links; OS presenters may ignore them.
	Actions []NoticeAction
	// OpenLabel names the button that opens Link where a presenter shows
	// one ("Buka" when empty).
	OpenLabel string
	// AccountID scopes transient desktop content to its signed-in recipient.
	AccountID string
	// Group gathers a burst of notices alike (several "Rapat baru" in one
	// Space within seconds) into one, where a presenter can: GroupLabel
	// names them ("rapat baru"), Space is where they are.
	Group, GroupLabel string
	Space             id.ID
	// Face is Ace's eyes for a NoticeAce (AceFaceHappy…); "" wide open.
	Face string
}

type NoticeAction struct{ Label, Link string }

// NoticePresenter is one way of showing notices: a browser popup, an OS
// notification, or, later, an island.
type NoticePresenter interface {
	Present(Notice) error
}

// Notifier is the device's notification layer.
type Notifier interface {
	// Permission is one of the BrowserPermission constants: what the
	// device lets Space show now. It is read every frame, so it must be
	// cheap. The desktop app answers granted when the OS notifications
	// could be registered, because Windows has no prompt.
	Permission() string
	// RequestPermission asks the user, where there is something to ask (the
	// browser's prompt). The window is drawn again when they answer.
	RequestPermission()
	NoticePresenter
}

// NotifierHint is a Notifier that can say where a denied permission is
// given again; the checklist shows it.
type NotifierHint interface {
	DeniedHint() string
}

// MultiNotifier is Base, whose permission it reports, also presenting
// through Extra (an island beside the OS notification). Errors of Extra
// do not hide Base's notice.
type MultiNotifier struct {
	Base  Notifier
	Extra []NoticePresenter
	// ReplaceBase tries Extra first. Only an accepted notice suppresses Base;
	// unavailable or failed presenters always fall back to the OS.
	ReplaceBase bool
}

func (m MultiNotifier) Permission() string { return m.Base.Permission() }
func (m MultiNotifier) RequestPermission() { m.Base.RequestPermission() }
func (m MultiNotifier) Present(n Notice) error {
	if m.ReplaceBase {
		for _, p := range m.Extra {
			if p.Present(n) == nil {
				return nil
			}
		}
		return m.Base.Present(n)
	}
	err := m.Base.Present(n)
	for _, p := range m.Extra {
		_ = p.Present(n)
	}
	return err
}

func (m MultiNotifier) DeniedHint() string {
	if h, ok := m.Base.(NotifierHint); ok {
		return h.DeniedHint()
	}
	return "Periksa izin notifikasi perangkat di Pengaturan."
}

// ShowNotice shows msg as a toast in Space on the next frame; device code
// calls it from the window's thread.
func (a *App) ShowNotice(msg string) { a.addNotice(msg) }

// noticeDeniedHint is where the device's permission is given again.
func (a *App) noticeDeniedHint() string {
	if h, ok := a.Notifier.(NotifierHint); ok {
		return h.DeniedHint()
	}
	return "Notifikasi diblokir di perangkat ini. Buka Pengaturan › Perangkat untuk mengizinkannya lagi."
}

// noticePermission is the device's permission, or unsupported without a
// notification layer.
func (a *App) noticePermission() string {
	if a.Notifier == nil {
		return BrowserPermissionUnsupported
	}
	return a.Notifier.Permission()
}

// watchNotificationPermission sends a test notice as soon as the device's
// permission becomes granted while Space is open, whatever asked for it:
// the checklist, the settings page, or the browser's site settings. Doing
// it right away lets the user see that notifications really appear.
func (a *App) watchNotificationPermission() {
	perm := a.noticePermission()
	before := a.onb.notifPerm
	a.onb.notifPerm = perm
	if (before != BrowserPermissionDefault && before != BrowserPermissionDenied) || perm != BrowserPermissionGranted || a.Notifier == nil {
		return
	}
	if err := a.Notifier.Present(Notice{Kind: NoticeTest, Tag: "space-izin", Title: "Notifikasi aktif ✓", Body: "Kamu akan dapat kabar tugas dan ping di sini."}); err != nil {
		a.addNotice("Notifikasi diizinkan, tetapi notifikasi uji belum muncul: " + err.Error())
	}
}
