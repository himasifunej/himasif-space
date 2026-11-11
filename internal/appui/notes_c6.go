package appui

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/client"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Catatan C6: Bagikan (Hanya divisi ini / Publik lewat tautan) and tags;
// C5: the new note filled from the next scheduled meeting.

// NoteSharingAPI is the server's public links and tags (0.4.2). Without
// it the share button and the tag field stay hidden.
type NoteSharingAPI interface {
	CreateNotePublicLink(context.Context, id.ID) (api.NotePublicLinkCreated, error)
	RevokeNotePublicLink(context.Context, id.ID) error
	SetNoteTags(context.Context, id.ID, []string) ([]string, error)
}

type noteSharingState struct {
	open bool
	busy bool
	err  string
	// url is the public link made in this session, shown for note only:
	// the server never sends a token again.
	url     string
	urlNote id.ID
	copied  bool
	tagText string
	tagBusy bool
	tagErr  string
	// fromMeeting fills a new Notulen from the next scheduled meeting, or
	// from meetingID when the Rapat tab asked for that meeting's notes.
	fromMeeting bool
	meetingID   id.ID
}

func (a *App) noteSharingService() (NoteSharingAPI, bool) {
	if c, ok := a.api.(*client.Client); ok {
		return c, true
	}
	s, ok := a.api.(NoteSharingAPI)
	return s, ok
}

// noteShareButton is the bar's "Publik" chip, or "Bagikan" for who may
// share; it opens the share popover.
func (a *App) noteShareButton(c *ui.Context, phone bool) {
	n := a.notes.note
	if n == nil {
		return
	}
	if _, ok := a.noteSharingService(); !ok {
		return
	}
	public := n.PublicLink != nil
	if !public && !n.Permissions.Share {
		return
	}
	t := c.Theme()
	label, text := "Bagikan catatan", "Bagikan"
	if public {
		label, text = "Publik — siapa pun dengan tautan bisa membaca", "Publik"
	}
	b := ui.ButtonBase(c.Key("note-share")).Label(label).Tooltip(label).Row().Height(26).Padding(0, 9).Gap(5).AlignItems(ui.Center).Radius(13).Shrink(0).Border(1, t.Border).TextColor(t.TextMuted)
	if public {
		b.Background(spaceMix(t.Accent, t.Surface, .88)).BorderColor(spaceMix(t.Accent, t.Surface, .55)).TextColor(t.Accent)
	} else if b.Hovered() {
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		ui.Icon(c, spaceIcon("globe")).Size(13, 13)
		if !phone || public {
			ui.Text(c, text).FontSize(12).FontWeight(500)
		}
	})
	onbPress(b)
	if b.Clicked() {
		a.notes.share.open = !a.notes.share.open
		a.notes.share.err, a.notes.share.copied = "", false
	}
	w, _ := c.Size()
	popover(c, b, &a.notes.share.open, min(float32(360), w-24), func() { a.noteSharePopover(c) })
}

func (a *App) noteSharePopover(c *ui.Context) {
	n := a.notes.note
	if n == nil {
		return
	}
	t := c.Theme()
	st := &a.notes.share
	public := n.PublicLink != nil
	ui.Text(c, "Bagikan catatan").Bold()
	option := func(key, title, detail string, on bool) ui.Element {
		o := ui.ButtonBase(c.Key(key)).Label(title).FillWidth().Row().Gap(10).Padding(10, 10).Radius(9).AlignItems(ui.Start).Border(1, t.Border).Disabled(st.busy || !n.Permissions.Share)
		if on {
			o.BorderColor(t.Accent).Background(spaceMix(t.Accent, t.Surface, .92))
		} else if o.Hovered() && n.Permissions.Share {
			o.Background(t.SurfaceHover)
		}
		o.FocusRing(o.FocusVisible()).Children(func() {
			ui.Box(c).Size(16, 16).Shrink(0).Radius(8).Center().Border(1.5, t.Border).Margin(2, 0, 0, 0).Children(func() {
				if on {
					ui.Box(c).Size(8, 8).Radius(4).Background(t.Accent)
				}
			})
			ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
				ui.Text(c, title).FontSize(13).FontWeight(600)
				ui.Text(c, detail).FontSize(12).TextColor(t.TextMuted)
			})
		})
		return o
	}
	if option("share-division", "Hanya divisi ini", "Anggota divisi pemilik yang bisa membaca. Bawaan setiap catatan.", !public).Clicked() && public {
		a.revokeNotePublicLink()
	}
	if option("share-public", "Publik lewat tautan", "Siapa pun yang memegang tautan bisa membaca tanpa login. Hanya-baca, tidak muncul di mesin pencari, tanpa nomor WA, NIM, email, atau data anggota.", public).Clicked() && !public {
		a.createNotePublicLink()
	}
	if public {
		ui.Column(c.Key("share-link")).FillWidth().Gap(6).Padding(4, 0, 0, 0).Children(func() {
			if st.urlNote == n.ID && st.url != "" {
				ui.Text(c, st.url).FontSize(12).Font(FontMono).Selectable().Padding(8, 10).Radius(8).Background(t.Background).Border(1, t.Border).FillWidth().MinWidth(0)
				copy := "Salin tautan"
				if st.copied {
					copy = "Tersalin ✓"
				}
				if onbPrimary(c, "Salin tautan publik", copy, true).Clicked() {
					c.WriteClipboard(st.url)
					st.copied = true
				}
			} else {
				muted(c, "Tautan hanya ditampilkan saat dibuat. Untuk menyalinnya lagi, buat tautan baru; tautan lama berhenti bekerja.").FontSize(12)
			}
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				if n.Permissions.Share && onbSecondary(c, "Buat tautan baru", "Buat tautan baru", true).Disabled(st.busy).Clicked() {
					a.createNotePublicLink()
				}
				if n.Permissions.Share && onbLink(c, "Matikan tautan publik", "Matikan").Clicked() {
					a.revokeNotePublicLink()
				}
			})
			l := n.PublicLink
			info := "Dibuat oleh " + firstNonEmpty(l.CreatedBy.DisplayName, "pengurus")
			if l.ViewCount > 0 {
				info += fmt.Sprintf(" · dibuka %d kali", l.ViewCount)
			}
			muted(c, info).FontSize(11)
		})
	}
	if !n.Permissions.Share {
		muted(c, "Hanya kadiv divisi atau penulis catatan yang bisa mengubah akses.").FontSize(11)
	}
	if st.err != "" {
		ui.Text(c, st.err).FontSize(12).TextColor(t.Danger)
	}
}

// shareWork runs a share call for the open note and applies its result.
func (a *App) shareWork(work func(NoteSharingAPI, id.ID) (func(), error)) {
	svc, ok := a.noteSharingService()
	n := a.notes.note
	if !ok || n == nil || a.notes.share.busy {
		return
	}
	gen, nid := a.gen, n.ID
	a.notes.share.busy, a.notes.share.err = true, ""
	a.async(func() func() {
		apply, err := work(svc, nid)
		return func() {
			if gen != a.gen {
				return
			}
			a.notes.share.busy = false
			if err != nil {
				a.notes.share.err = a.failed(err)
				return
			}
			if a.notes.note != nil && a.notes.note.ID == nid {
				apply()
				a.replaceNote(*a.notes.note)
			}
		}
	})
}

func (a *App) createNotePublicLink() {
	a.shareWork(func(svc NoteSharingAPI, nid id.ID) (func(), error) {
		made, err := svc.CreateNotePublicLink(context.Background(), nid)
		return func() {
			link := made.Link
			a.notes.note.PublicLink = &link
			a.notes.share.url, a.notes.share.urlNote, a.notes.share.copied = made.URL, nid, false
		}, err
	})
}

func (a *App) revokeNotePublicLink() {
	a.shareWork(func(svc NoteSharingAPI, nid id.ID) (func(), error) {
		err := svc.RevokeNotePublicLink(context.Background(), nid)
		return func() {
			a.notes.note.PublicLink = nil
			a.notes.share.url = ""
		}, err
	})
}

// noteTagsSection is the panel's Tag: chips, and for writers a field.
func (a *App) noteTagsSection(c *ui.Context) {
	n := a.notes.note
	if n == nil {
		return
	}
	_, ok := a.noteSharingService()
	if !ok || (len(n.Tags) == 0 && !n.Permissions.Manage) {
		return
	}
	t := c.Theme()
	st := &a.notes.share
	notePanelSection(c, "sec-tag", "Tag", func() {
		ui.Row(c.Key("tags")).FillWidth().Wrap().Gap(6).Children(func() {
			for _, tag := range n.Tags {
				chip := ui.Row(c.Key("tag-"+tag)).Height(24).Padding(0, 4, 0, 9).Gap(2).Radius(12).AlignItems(ui.Center).Border(1, t.Border).Transition(onbPop)
				chip.Children(func() {
					ui.Text(c, "#"+tag).FontSize(12).TextColor(t.TextMuted)
					if n.Permissions.Manage {
						x := ui.ButtonBase(c.Key("x")).Label("Hapus tag "+tag).Size(18, 18).Center().Radius(9).TextColor(faint(t))
						if x.Hovered() {
							x.Background(t.SurfaceHover).TextColor(t.Text)
						}
						x.Children(func() { ui.Icon(c, spaceIcon("close")).Size(10, 10) })
						if x.Clicked() {
							a.setNoteTags(slices.DeleteFunc(slices.Clone(n.Tags), func(s string) bool { return s == tag }))
						}
					}
				})
			}
			if len(n.Tags) == 0 {
				muted(c, "Belum ada tag.").FontSize(12)
			}
		})
		if n.Permissions.Manage {
			in := ui.TextInput(c.Key("tag-input"), &st.tagText).Label("Tambah tag").Placeholder("Tambah tag, lalu Enter").FillWidth().Disabled(st.tagBusy)
			if in.Submitted() && strings.TrimSpace(st.tagText) != "" {
				next := append(slices.Clone(n.Tags), strings.Split(st.tagText, ",")...)
				a.setNoteTags(next)
			}
		}
		if st.tagErr != "" {
			ui.Text(c, st.tagErr).FontSize(12).TextColor(t.Danger)
		}
	})
}

func (a *App) setNoteTags(tags []string) {
	norm, err := api.NormalizeNoteTags(tags)
	if err != nil {
		a.notes.share.tagErr = err.Error()
		return
	}
	svc, ok := a.noteSharingService()
	n := a.notes.note
	if !ok || n == nil {
		return
	}
	gen, nid := a.gen, n.ID
	a.notes.share.tagBusy, a.notes.share.tagErr = true, ""
	a.async(func() func() {
		saved, err := svc.SetNoteTags(context.Background(), nid, norm)
		return func() {
			if gen != a.gen {
				return
			}
			a.notes.share.tagBusy = false
			if err != nil {
				a.notes.share.tagErr = a.failed(err)
				return
			}
			a.notes.share.tagText = ""
			if a.notes.note != nil && a.notes.note.ID == nid {
				a.notes.note.Tags = saved
				a.replaceNote(*a.notes.note)
			}
		}
	})
}

// nextNoteMeeting is the space's next scheduled meeting that has no note
// yet: from twelve hours ago on, the earliest.
func (a *App) nextNoteMeeting(now time.Time) *api.Meeting {
	if a.meetings.space != a.notes.space {
		return nil
	}
	var best *api.Meeting
	for i := range a.meetings.items {
		m := &a.meetings.items[i]
		if m.Cancelled || m.NoteID != nil || m.StartsAt.Before(now.Add(-12*time.Hour)) {
			continue
		}
		if best == nil || m.StartsAt.Before(best.StartsAt) {
			best = m
		}
	}
	return best
}

// noteMeetingChoice is the create popover's "Isi dari rapat" for Notulen.
func (a *App) noteMeetingChoice(c *ui.Context) *api.Meeting {
	if a.notes.templateKind != api.TemplateMeeting || a.notes.templateID != id.Nil {
		return nil
	}
	a.ensureMeetings()
	m := a.nextNoteMeeting(c.Now())
	if m == nil {
		muted(c, "Belum ada rapat terjadwal tanpa notulen di ruang ini.").FontSize(11)
		return nil
	}
	t := c.Theme()
	st := &a.notes.share
	b := ui.ButtonBase(c.Key("from-meeting")).Label("Isi dari rapat "+m.Title).FillWidth().Row().Gap(8).Padding(8, 10).Radius(8).AlignItems(ui.Center).Border(1, t.Border)
	if st.fromMeeting {
		b.BorderColor(t.Accent)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		onbCheck(c, st.fromMeeting, 18)
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Text(c, "Isi dari rapat terjadwal").FontSize(12).FontWeight(600)
			ui.Text(c, m.Title+" · "+noteMeetingWhen(m, true)).FontSize(11).TextColor(t.TextMuted).MaxLines(2)
		})
	})
	if b.Clicked() {
		st.fromMeeting = !st.fromMeeting
	}
	if st.fromMeeting {
		return m
	}
	return nil
}
