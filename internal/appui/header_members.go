package appui

import (
	"context"
	"fmt"
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"slices"
	"strings"
)

type headerMembersState struct {
	space           id.ID
	loading, loaded bool
	err             string
	items           []api.PersonRef
}

func (a *App) headerMembers(c *ui.Context) {
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
	b := ui.ButtonBase(c.Key("space-members")).Label("Anggota Space").Tooltip("Anggota Space").Row().Gap(4).Padding(5, 8).MinHeight(36).Radius(9).AlignItems(ui.Center)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		for i, person := range m.items {
			if i == 3 {
				break
			}
			ui.Box(c.Key(person.ID)).Size(27, 27).Radius(14).Center().Background(t.SurfaceHover).Border(1, t.Border).Children(func() { ui.Text(c, personInitials(person.DisplayName)).FontSize(10).Bold() })
		}
		if len(m.items) == 0 {
			ui.Icon(c, spaceIcon("users")).Size(18, 18)
		}
		ui.Text(c, fmt.Sprint(len(m.items))).FontSize(11).TextColor(t.TextMuted)
		ui.Icon(c, spaceIcon("down")).Size(12, 12)
	})
	if b.Clicked() {
		a.chrome.membersOpen = !a.chrome.membersOpen
		a.chrome.searchOpen = false
		a.chrome.notificationsOpen = false
		a.chrome.profileOpen = false
	}
	popover(c, b, &a.chrome.membersOpen, 280, func() {
		ui.Text(c, "Anggota Space").Bold()
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
					ui.Row(c.Key(person.ID)).Padding(8).Gap(10).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(28, 28).Radius(14).Center().Background(t.SurfaceHover).Children(func() { ui.Text(c, personInitials(person.DisplayName)).FontSize(10) })
						ui.Text(c, person.DisplayName).FontSize(12).Grow(1).MinWidth(0)
					})
				}
			})
		})
	})
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

func personInitials(name string) string {
	out := ""
	for _, part := range strings.Fields(name) {
		out += strings.ToUpper(string([]rune(part)[0]))
		if len([]rune(out)) == 2 {
			break
		}
	}
	return out
}
