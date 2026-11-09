package appui

import (
	"context"
	"fmt"
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/workspaceui"
	"slices"
)

type headerMembersState struct {
	space           id.ID
	loading, loaded bool
	err             string
	items           []api.PersonRef
}

// headerMembers is the canvas' member chip: up to three overlapping
// avatars and "N anggota" (the count alone when the topbar is narrow).
func (a *App) headerMembers(c *ui.Context, showText bool) {
	sp := a.selectedSpace()
	if a.router.Path() != PathSpaces || sp == nil || !slices.Contains(sp.Capabilities, api.CapMembersRead) {
		return
	}
	if a.chrome.members.space != sp.ID {
		a.chrome.members = headerMembersState{space: sp.ID}
		a.chrome.membersOpen = false
	}
	m := &a.chrome.members
	if !m.loading && !m.loaded && m.err == "" {
		a.loadHeaderMembers()
	}
	t := c.Theme()
	b := ui.ButtonBase(c.Key("space-members")).Label("Anggota Space").Tooltip("Anggota Space").Row().Gap(6).Shrink(0).Padding(3, 8, 3, 4).MinHeight(34).Radius(20).Border(1, t.Border).AlignItems(ui.Center)
	if b.Hovered() || a.chrome.membersOpen {
		b.Background(t.SurfaceHover)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		shown := min(len(m.items), 3)
		if shown == 0 {
			ui.Icon(c, spaceIcon("users")).Size(16, 16).Margin(0, 0, 0, 4)
		} else {
			// Avatars overlap by 7 px, each ringed in the surface color.
			ui.Box(c).Size(26+float32(shown-1)*19, 26).Shrink(0).Children(func() {
				for i := 0; i < shown; i++ {
					person := m.items[i]
					avatar(c.Key(person.ID), person.DisplayName, 26).Absolute().Left(float32(i)*19).Top(0).Border(2, t.Surface)
				}
			})
		}
		n, known := a.headerMemberCount(sp)
		text := "Anggota"
		switch {
		case known && showText:
			text = fmt.Sprintf("%d anggota", n)
		case known:
			text = fmt.Sprint(n)
		case !showText:
			text = ""
		}
		if text != "" {
			ui.Text(c, text).FontSize(12).TextColor(t.TextMuted).MaxLines(1)
		}
	})
	if b.Clicked() {
		a.chrome.membersOpen = !a.chrome.membersOpen
		a.chrome.searchOpen = false
		a.chrome.notificationsOpen = false
		a.chrome.profileOpen = false
	}
	popover(c, b, &a.chrome.membersOpen, 280, func() {
		ui.Text(c, "Anggota "+sp.Name).FontSize(12).TextColor(t.TextMuted).Padding(4, 8, 6, 8)
		if m.loading {
			muted(c, "Memuat anggota…")
		}
		if m.err != "" {
			muted(c, m.err)
			if spaceButton(c, "Muat ulang anggota", "refresh", false).Clicked() {
				a.loadHeaderMembers()
			}
		}
		ui.Scroll(c).FillWidth().MaxHeight(280).Children(func() {
			ui.Column(c).FillWidth().Gap(4).Children(func() {
				for _, person := range m.items {
					ui.Row(c.Key(person.ID)).Padding(6, 8).Gap(10).AlignItems(ui.Center).Children(func() {
						avatar(c, person.DisplayName, 26)
						ui.Text(c, person.DisplayName).FontSize(13).Grow(1).MinWidth(0).MaxLines(1)
					})
				}
			})
		})
		if canPing(sp) && spaceButton(c, "Ping semua anggota", "send", false).FillWidth().Clicked() {
			a.chrome.membersOpen = false
			a.chrome.pingOpen = true
		}
	})
}

// headerMemberCount is the number the member chip shows, and whether it is
// known. In a division Space whose member table is loaded it is the
// table's count, so the chip and the Anggota page agree; otherwise it is
// the count of the Space's people (meeting candidates). Before either has
// loaded, or after a failure, it is unknown: no "0 anggota".
func (a *App) headerMemberCount(sp *api.Space) (int, bool) {
	if m := &a.access.members; sp.Kind == api.SpaceDivision && len(sp.Divisions) == 1 && m.loaded && m.division == sp.Divisions[0].ID {
		return len(m.members), true
	}
	if a.chrome.members.loaded {
		return len(a.chrome.members.items), true
	}
	return 0, false
}

func (a *App) loadHeaderMembers() {
	svc, ok := a.meetingsService()
	if !ok {
		a.chrome.members.err = "Daftar anggota belum tersedia."
		return
	}
	sid, gen := a.chrome.members.space, a.gen
	a.chrome.members.loading = true
	a.async(func() func() {
		items, err := svc.MeetingCandidates(context.Background(), sid)
		return func() {
			if gen != a.gen || a.selectedSpaceID() != sid || a.chrome.members.space != sid {
				return
			}
			m := &a.chrome.members
			m.loading = false
			if err != nil {
				m.items = nil
				m.err = a.failed(err)
				return
			}
			m.loaded = true
			m.err = ""
			m.items = items
		}
	})
}

// personInitials is the avatar text of a person: "[DUMMY] Aruna Desain"
// gives "AD".
func personInitials(name string) string { return workspaceui.Initials(name) }
