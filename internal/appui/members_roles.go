package appui

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

// Role kustom has its own dialog, opened from the Anggota page's header,
// the Kewenangan role card and the access drawer, instead of a long form
// under the table (pilot 0.4.4). It lists the roles; a role is made from
// an example or from nothing, with what it allows as cards with an icon
// and a line each in plain words.

// roleCapChoice is a permission a custom role can give.
type roleCapChoice struct{ cap, label, icon, desc string }

// roleCapChoices are operationalRoleChoices, with an icon and what each
// allows in a line.
var roleCapChoices = []roleCapChoice{
	{api.CapTasksManage, "Kelola tugas", "task", "Membuat, mengubah, dan membagi tugas."},
	{api.CapResultsReview, "Tinjau hasil", "check", "Menerima hasil kerja atau meminta revisi."},
	{api.CapPingSend, "Kirim ping", "bell", "Mengingatkan anggota lewat WhatsApp."},
	{api.CapWhatsAppManage, "Kelola WhatsApp", "whatsapp", "Mengatur bot dan grup WhatsApp divisi."},
	{api.CapNotesManage, "Kelola catatan", "note", "Menulis dan merapikan catatan rapat."},
	{api.CapSOPRead, "Baca evaluasi SOP", "book", "Melihat hasil evaluasi SOP divisi."},
	{api.CapSOPReview, "Tinjau temuan SOP", "eye", "Menanggapi temuan evaluasi SOP."},
}

// rolePreset is an example to start a role from.
type rolePreset struct {
	name, icon string
	caps       []string
}

var rolePresets = []rolePreset{
	{"Koordinator tugas", "task", []string{api.CapTasksManage, api.CapResultsReview, api.CapPingSend}},
	{"Notulis", "note", []string{api.CapNotesManage}},
	{"Admin WhatsApp", "whatsapp", []string{api.CapWhatsAppManage, api.CapPingSend}},
	{"Pemeriksa SOP", "book", []string{api.CapSOPRead, api.CapSOPReview}},
}

// roleDialogState is the dialog: open, on its list or editing a role
// (roleEditing, or a new one), or asking to delete one.
type roleDialogState struct {
	open    bool
	editing bool
	// deleting is the role whose deletion the dialog asks about.
	deleting id.ID
}

// openRoles opens the dialog on its list, or on a new role when there is
// none yet.
func (a *App) openRoles() {
	m := &a.access.members
	m.roleDialog = roleDialogState{open: true}
	if len(m.roles) == 0 {
		a.editRole(nil)
	}
}

// editRole puts the dialog on r's form, or a new role's for nil.
func (a *App) editRole(r *api.CustomRole) {
	m := &a.access.members
	m.roleDialog.editing, m.roleDialog.deleting = true, id.Nil
	m.roleEditing, m.roleName, m.roleCaps = id.Nil, "", nil
	if r != nil {
		m.roleEditing, m.roleName, m.roleCaps = r.ID, r.Name, slices.Clone(r.Capabilities)
	}
}

// roleUsers counts the members holding custom role rid.
func (a *App) roleUsers(rid id.ID) int {
	n := 0
	for _, member := range a.access.members.members {
		if slices.Contains(member.CustomRoleIDs, rid) {
			n++
		}
	}
	return n
}

// memberRolesDialog is the Role kustom dialog.
func (a *App) memberRolesDialog(c *ui.Context) {
	m := &a.access.members
	if !m.roleDialog.open {
		return
	}
	if !a.memberCan(api.CapRolesManage) {
		m.roleDialog.open = false
		return
	}
	t := c.Theme()
	w, h := c.Size()
	phone := w < mobileMaxWidth
	ui.DialogBase(c.Key("roles-dialog"), &m.roleDialog.open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.35))
		panel.Width(min(float32(620), w-32)).MaxHeight(h-48).Radius(14).Background(t.Surface).Border(1, t.Border).Label("Dialog role kustom").Children(func() {
			ui.Row(c).FillWidth().Padding(18, 20, 12, 20).Gap(12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
				ui.Box(c).Size(36, 36).Shrink(0).Radius(10).Center().Background(accentSoft(t)).Children(func() {
					ui.Icon(c, spaceIcon("shield")).Size(18, 18).TextColor(t.Accent)
				})
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					title := "Role kustom"
					switch {
					case m.roleDialog.editing && m.roleEditing != id.Nil:
						title = "Ubah role"
					case m.roleDialog.editing:
						title = "Role baru"
					}
					ui.Text(c, title).FontSize(16).Bold()
					ui.Text(c, "Izin tambahan untuk anggota tertentu, di atas role dasarnya.").FontSize(12).TextColor(t.TextMuted)
				})
				if spaceIconButton(c, "Tutup role kustom", "close").Clicked() {
					m.roleDialog.open = false
				}
			})
			ui.Scroll(c.Key("roles-body")).Grow(1).MinHeight(0).FillWidth().Children(func() {
				ui.Column(c).FillWidth().Padding(16, 20).Gap(14).Children(func() {
					if m.err != "" {
						ui.Text(c, m.err).FontSize(12).TextColor(t.Danger)
					}
					switch {
					case m.roleDialog.deleting != id.Nil:
						a.roleDeleteAsk(c)
					case m.roleDialog.editing:
						a.roleForm(c, phone)
					default:
						a.roleList(c)
					}
				})
			})
			if m.roleDialog.editing && m.roleDialog.deleting == id.Nil {
				a.roleFormFooter(c)
			}
		})
	})
}

// roleList lists the roles, each with its permissions as icons, and the
// button to make one.
func (a *App) roleList(c *ui.Context) {
	t := c.Theme()
	m := &a.access.members
	if len(m.roles) == 0 {
		ui.Column(c).FillWidth().Padding(20, 0).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, spaceIcon("shield")).Size(28, 28).TextColor(faint(t))
			ui.Text(c, "Belum ada role kustom").FontSize(14).Bold()
			muted(c, "Contoh: beri satu anggota izin mengelola tugas tanpa menjadikannya 45.")
		})
	}
	for _, r := range m.roles {
		ui.Row(c.Key(r.ID)).FillWidth().MinWidth(0).Padding(12, 14).Gap(12).AlignItems(ui.Center).Radius(10).Border(1, t.Border).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(6).Children(func() {
				ui.Text(c, r.Name).FontSize(14).FontWeight(600).MaxLines(2)
				ui.Row(c).Gap(6).AlignItems(ui.Center).Wrap().MinWidth(0).Children(func() {
					for _, ch := range roleCapChoices {
						if slices.Contains(r.Capabilities, ch.cap) {
							ui.Row(c.Key(ch.cap)).Gap(4).Height(22).Padding(0, 8).Radius(6).AlignItems(ui.Center).Background(t.SurfacePressed).Children(func() {
								ui.Icon(c, spaceIcon(ch.icon)).Size(12, 12).TextColor(t.TextMuted)
								ui.Text(c, ch.label).FontSize(11).TextColor(t.TextMuted).MaxLines(1)
							})
						}
					}
				})
				used := "Belum dipakai"
				if n := a.roleUsers(r.ID); n > 0 {
					used = fmt.Sprintf("Dipakai %d anggota", n)
				}
				ui.Text(c, used).FontSize(11).TextColor(faint(t))
			})
			if spaceIconButton(c, "Edit role "+r.Name, "edit").Disabled(a.access.members.busy).Clicked() {
				a.editRole(&r)
			}
			if spaceIconButton(c, "Hapus role "+r.Name, "close").Disabled(a.access.members.busy).Clicked() {
				m.roleDialog.deleting = r.ID
			}
		})
	}
	ui.Row(c).FillWidth().Children(func() {
		if primarySpaceButton(c, "Buat role baru", "plus").Disabled(m.busy).Clicked() {
			a.editRole(nil)
		}
	})
}

// roleForm is a role's name, the examples to start from, and what it
// allows as cards to tick.
func (a *App) roleForm(c *ui.Context, phone bool) {
	t := c.Theme()
	m := &a.access.members
	if m.roleEditing == id.Nil {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			ui.Text(c, "Mulai dari contoh").FontSize(13).Bold()
			ui.Row(c).FillWidth().Gap(8).Wrap().Children(func() {
				for _, p := range rolePresets {
					on := m.roleName == p.name
					b := ui.ButtonBase(c.Key("preset-"+p.name)).Label("Contoh "+p.name).Row().AlignItems(ui.Center).Gap(6).Height(32).Padding(0, 12).Radius(16).Border(1, t.Border).TextColor(t.TextMuted).Shrink(0)
					if on {
						b.Background(accentSoft(t)).BorderColor(spaceMix(t.Accent, t.Background, .45)).TextColor(t.Accent)
					} else if b.Hovered() {
						b.Background(t.SurfaceHover).TextColor(t.Text)
					}
					b.FocusRing(b.FocusVisible()).Children(func() {
						ui.Icon(c, spaceIcon(p.icon)).Size(14, 14)
						ui.Text(c, p.name).FontSize(13)
					})
					if b.Clicked() {
						m.roleName, m.roleCaps = p.name, slices.Clone(p.caps)
					}
				}
			})
		})
	}
	ui.TextInput(c.Key("role-name"), &m.roleName).Label("Nama role kustom").Placeholder("Misalnya: Koordinator acara").FillWidth().Disabled(m.busy)
	ui.Text(c, "Yang boleh dilakukan").FontSize(13).Bold()
	cols := 2
	if phone {
		cols = 1
	}
	for i := 0; i < len(roleCapChoices); i += cols {
		ui.Row(c.Key(i)).FillWidth().Gap(8).AlignItems(ui.Stretch).Children(func() {
			for j := i; j < i+cols; j++ {
				if j >= len(roleCapChoices) {
					ui.Box(c.Key(j)).Grow(1).Basis(0)
					continue
				}
				ch := roleCapChoices[j]
				on := slices.Contains(m.roleCaps, ch.cap)
				if roleCapCard(c, ch, on).Disabled(m.busy).Clicked() {
					m.roleCaps = toggleString(m.roleCaps, ch.cap)
				}
			}
		})
	}
	ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, spaceIcon("lock")).Size(13, 13).TextColor(faint(t)).Shrink(0)
		ui.Text(c, "Mengatur anggota, role, dan 45 tetap hanya kadiv.").FontSize(12).TextColor(faint(t))
	})
}

// roleCapCard is a permission to tick: its icon, name and line, and a
// check when on.
func roleCapCard(c *ui.Context, ch roleCapChoice, on bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c.Key("cap-"+ch.cap)).Label(ch.label).Row().Grow(1).Basis(0).MinWidth(0).Padding(10, 12).Gap(10).AlignItems(ui.Center).Radius(10).Border(1, t.Border).TextColor(t.Text)
	if on {
		b.Background(accentSoft(t)).BorderColor(spaceMix(t.Accent, t.Background, .45))
	} else if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.FocusRing(b.FocusVisible()).Children(func() {
		tile := ui.Box(c).Size(32, 32).Shrink(0).Radius(8).Center().Background(t.SurfacePressed)
		ink := t.TextMuted
		if on {
			tile.Background(t.Accent)
			ink = t.AccentText
		}
		tile.Children(func() { ui.Icon(c, spaceIcon(ch.icon)).Size(16, 16).TextColor(ink) })
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Text(c, ch.label).FontSize(13).FontWeight(600).MaxLines(1)
			ui.Text(c, ch.desc).FontSize(12).TextColor(t.TextMuted).LineHeight(1.4)
		})
		box := ui.Box(c).Size(18, 18).Shrink(0).Radius(9).Border(1, t.Border).Center()
		if on {
			box.Background(t.Accent).BorderColor(t.Accent).Children(func() {
				ui.Icon(c, spaceIcon("check")).Size(12, 12).TextColor(t.AccentText)
			})
		}
	})
	return b
}

// roleFormFooter counts what the role allows and saves it.
func (a *App) roleFormFooter(c *ui.Context) {
	t := c.Theme()
	m := &a.access.members
	ui.Row(c).FillWidth().Padding(12, 20).Gap(8).AlignItems(ui.Center).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		hint := fmt.Sprintf("%d izin dipilih", len(m.roleCaps))
		switch {
		case strings.TrimSpace(m.roleName) == "":
			hint = "Beri nama role dulu"
		case len(m.roleCaps) == 0:
			hint = "Pilih minimal satu izin"
		}
		ui.Text(c, hint).FontSize(12).TextColor(t.TextMuted).Grow(1).MinWidth(0)
		if spaceButton(c, "Batal", "", false).Clicked() {
			if len(m.roles) == 0 {
				m.roleDialog.open = false
			}
			m.roleDialog.editing = false
		}
		label := "Buat role"
		if m.roleEditing != id.Nil {
			label = "Simpan role"
		}
		ready := strings.TrimSpace(m.roleName) != "" && len(m.roleCaps) > 0
		if primarySpaceButton(c, label, "check").Disabled(m.busy || !ready).Clicked() {
			req := api.RoleRequest{Name: strings.TrimSpace(m.roleName), Capabilities: append([]string{}, m.roleCaps...)}
			rid := m.roleEditing
			m.roleDialog.editing = false
			a.memberWrite(api.CapRolesManage, "Role "+req.Name+" disimpan.", func(s WorkspaceAPI, p, d id.ID) error {
				if rid == id.Nil {
					_, err := s.CreateRole(context.Background(), p, d, req)
					return err
				}
				_, err := s.UpdateRole(context.Background(), p, d, rid, req)
				return err
			})
		}
	})
}

// roleDeleteAsk asks before deleting a role, saying who loses it.
func (a *App) roleDeleteAsk(c *ui.Context) {
	t := c.Theme()
	m := &a.access.members
	rid := m.roleDialog.deleting
	i := slices.IndexFunc(m.roles, func(r api.CustomRole) bool { return r.ID == rid })
	if i < 0 {
		m.roleDialog.deleting = id.Nil
		return
	}
	r := m.roles[i]
	ui.Column(c).FillWidth().Gap(10).Padding(14, 16).Radius(10).Border(1, spaceMix(t.Danger, t.Surface, .55)).Background(spaceMix(t.Danger, t.Surface, .9)).Children(func() {
		ui.Text(c, "Hapus role "+r.Name+"?").FontSize(14).Bold().TextColor(t.Danger)
		text := "Belum ada anggota yang memakainya."
		if n := a.roleUsers(r.ID); n > 0 {
			text = fmt.Sprintf("%d anggota kehilangan izin dari role ini. Role dasar mereka tetap.", n)
		}
		ui.Text(c, text).FontSize(13).TextColor(t.TextMuted).LineHeight(1.5)
		ui.Row(c).FillWidth().Gap(8).Justify(ui.End).Children(func() {
			if spaceButton(c, "Batal hapus", "", false).Clicked() {
				m.roleDialog.deleting = id.Nil
			}
			del := spaceButton(c, "Hapus role", "close", false).Background(t.Danger).TextColor(t.Background).Disabled(m.busy)
			if del.Clicked() {
				m.roleDialog.deleting = id.Nil
				a.memberWrite(api.CapRolesManage, "Role "+r.Name+" dihapus.", func(s WorkspaceAPI, p, d id.ID) error { return s.DeleteRole(context.Background(), p, d, rid) })
			}
		})
	})
}
