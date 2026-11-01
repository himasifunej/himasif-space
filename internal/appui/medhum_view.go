package appui

import (
	"context"
	"github.com/egoist/mygo/ui"
	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
	"github.com/himasifunej/himasif-space/internal/timeutil"
	"slices"
	"strings"
	"time"
)

type medhumDraft struct {
	id                                                   id.ID
	version                                              int64
	title, date, clock, concept, caption, trello, reason string
	humas, design                                        id.ID
	calendar, assignment                                 bool
}
type medhumState struct {
	policy                    api.MedhumPolicy
	postingClock, designClock string
	policyDirty               bool
	seq                       uint64
	space                     id.ID
	loaded, loading, busy     bool
	items                     []api.MedhumContent
	humans, designers         []api.Member
	draft                     *medhumDraft
	err, filter               string
}

func (a *App) ensureMedhum() {
	sid := a.selectedSpaceID()
	if a.medhum.space != sid {
		a.medhum = medhumState{space: sid, seq: a.medhum.seq + 1}
	}
	if sid != id.Nil && !a.medhum.loaded && !a.medhum.loading && a.medhum.err == "" {
		a.loadMedhum()
	}
}
func (a *App) loadMedhum() {
	cl := a.notificationClient()
	sp := a.selectedSpace()
	if cl == nil || sp == nil || sp.Code != "medhum" || a.medhum.loading {
		return
	}
	sid, pid, gen, seq := sp.ID, sp.PeriodID, a.gen, a.medhum.seq
	divisions := slices.Clone(sp.Divisions)
	roles := slices.Clone(sp.Roles)
	a.medhum.loading = true
	a.async(func() func() {
		items, err := cl.MedhumContents(context.Background(), sid)
		policy, policyErr := cl.MedhumPolicy(context.Background(), sid)
		if err == nil {
			err = policyErr
		}
		var humans, designers []api.Member
		if err == nil {
			for _, d := range divisions {
				writable := slices.ContainsFunc(roles, func(role api.Access) bool {
					return role.DivisionID == d.ID && slices.Contains(role.Capabilities, api.CapTasksManage) && !role.ReadOnly
				})
				if !writable {
					continue
				}
				members, e := cl.Members(context.Background(), pid, d.ID)
				if e != nil {
					continue
				}
				if d.Slug == "humas" {
					humans = members
				} else if d.Slug == "mediatek" {
					designers = members
				}
			}
		}
		return func() {
			if gen != a.gen || seq != a.medhum.seq || sid != a.selectedSpaceID() {
				return
			}
			a.medhum.loading = false
			if err != nil {
				a.medhum.err = a.failed(err)
				return
			}
			a.medhum.items = items
			a.medhum.policy = policy
			if !a.medhum.policyDirty {
				a.medhum.postingClock = ""
				a.medhum.designClock = ""
				if policy.PostingClock != nil {
					a.medhum.postingClock = *policy.PostingClock
				}
				if policy.DesignClock != nil {
					a.medhum.designClock = *policy.DesignClock
				}
			}
			a.medhum.humans = humans
			a.medhum.designers = designers
			a.medhum.loaded = true
			a.medhum.err = ""
		}
	})
}
func (a *App) beginMedhum(item *api.MedhumContent) {
	draft := &medhumDraft{date: time.Now().Format("2006-01-02")}
	if len(a.medhum.humans) > 0 {
		draft.humas = a.medhum.humans[0].UserID
	}
	if item != nil {
		draft.id = item.ID
		draft.version = item.Version
		draft.title = item.Title
		draft.humas = item.HumasPJID
		draft.concept = item.ConceptURL
		draft.caption = item.Caption
		draft.trello = item.TrelloCardID
		draft.calendar = item.CanEditCalendar
		draft.assignment = item.CanAssignDesign
		if item.DesignPJID != nil {
			draft.design = *item.DesignPJID
		}
		if item.PostingDate != nil {
			draft.date = item.PostingDate.String()
		} else if item.PostingAt != nil {
			loc, _ := timeutil.LoadZone(a.period().TimeZone)
			draft.date = item.PostingAt.In(loc).Format("2006-01-02")
			draft.clock = item.PostingAt.In(loc).Format("15:04")
		}
	} else {
		draft.calendar = true
	}
	a.medhum.draft = draft
}
func (a *App) saveMedhum() {
	d := a.medhum.draft
	cl := a.notificationClient()
	if d == nil || cl == nil || a.medhum.busy || a.offlineMode() {
		return
	}
	var date *timeutil.Date
	var at *time.Time
	if d.calendar {
		parsed, err := timeutil.ParseDate(d.date)
		if err != nil {
			a.medhum.err = "Isi tanggal posting YYYY-MM-DD."
			return
		}
		date = &parsed
		if strings.TrimSpace(d.clock) != "" {
			loc, _ := timeutil.LoadZone(a.period().TimeZone)
			t, err := time.ParseInLocation("2006-01-02 15:04", d.date+" "+d.clock, loc)
			if err != nil {
				a.medhum.err = "Jam posting harus HH:MM, atau kosong untuk tanggal saja."
				return
			}
			t = t.UTC()
			at = &t
			date = nil
		}
	}
	create := api.MedhumCreateRequest{Title: d.title, PostingDate: date, PostingAt: at, HumasPJID: d.humas, ConceptURL: d.concept, Caption: d.caption}
	update := api.MedhumUpdateRequest{ExpectedVersion: d.version, Reason: d.reason}
	if d.calendar {
		title, caption, concept, trello, humas := d.title, d.caption, d.concept, d.trello, d.humas
		update.Title = &title
		update.Caption = &caption
		update.ConceptURL = &concept
		update.TrelloCardID = &trello
		update.HumasPJID = &humas
		update.PostingDate = date
		update.PostingAt = at
	}
	if d.assignment && d.design != id.Nil {
		design := d.design
		update.DesignPJID = &design
	}
	gen, seq, sid, cid := a.gen, a.medhum.seq, a.medhum.space, d.id
	a.medhum.busy = true
	a.async(func() func() {
		var err error
		if cid == id.Nil {
			_, err = cl.CreateMedhum(context.Background(), sid, create)
		} else {
			_, err = cl.UpdateMedhum(context.Background(), cid, update)
		}
		return func() {
			if gen != a.gen || seq != a.medhum.seq || sid != a.selectedSpaceID() {
				return
			}
			a.medhum.busy = false
			if err != nil {
				a.medhum.err = a.failed(err)
				return
			}
			a.medhum.draft = nil
			a.loadMedhum()
		}
	})
}
func (a *App) medhumView(c *ui.Context) {
	a.ensureMedhum()
	pageBody(c, "Kalender konten Medhum", func() {
		muted(c, "Tanggal Humas adalah jadwal posting. Deadline desain diturunkan H-2 sekali; kedua divisi tetap mempunyai tugas dan tanggung jawabnya sendiri.")
		ui.Row(c).Wrap().Gap(8).Children(func() {
			if spaceButton(c, "Muat ulang kalender", "refresh", false).Disabled(a.medhum.loading || a.medhum.busy).Clicked() {
				a.loadMedhum()
			}
			if spaceButton(c, "Tambah konten", "plus", true).Disabled(len(a.medhum.humans) == 0 || a.medhum.busy || a.offlineMode()).Clicked() {
				a.beginMedhum(nil)
			}
			if spaceButton(c, "Sumber kalender", "link", false).Clicked() {
				a.chrome.tab = "Sumber kalender"
			}
		})
		ui.TextInput(c, &a.medhum.filter).Label("Cari konten").FillWidth()
		a.medhumPolicyForm(c)
		if a.medhum.err != "" {
			ui.Text(c, a.medhum.err).TextColor(c.Theme().Danger)
		}
		if a.medhum.draft != nil {
			a.medhumForm(c)
		}
		for _, item := range a.medhum.items {
			item := item
			if q := strings.ToLower(strings.TrimSpace(a.medhum.filter)); q != "" && !strings.Contains(strings.ToLower(item.Title), q) {
				continue
			}
			ui.Column(c.Key(item.ID)).FillWidth().Padding(16).Gap(8).Border(1, c.Theme().Border).Radius(10).Children(func() {
				ui.Text(c, item.Title).Bold().FontSize(18)
				muted(c, "Jadwal posting: "+taskDate(item.PostingDate, item.PostingAt, a.period().TimeZone))
				muted(c, "Deadline desain: "+taskDate(item.DesignDate, item.DesignAt, a.period().TimeZone))
				ui.Text(c, "PJ Humas: "+item.HumasPJName)
				name := item.DesignPJName
				if item.DesignPJID == nil {
					name = "Belum ditetapkan kadiv Mediatek"
				}
				ui.Text(c, "PJ desain: "+name)
				if item.ConceptURL != "" {
					muted(c, item.ConceptURL)
				}
				if item.Caption != "" {
					ui.Text(c, item.Caption)
				}
				if item.ConceptSubmittedAt != nil {
					muted(c, "Konsep diserahkan: "+taskTimestamp(*item.ConceptSubmittedAt, a.period().TimeZone))
				}
				if item.PublishedAt != nil {
					muted(c, "Publikasi dicatat: "+taskTimestamp(*item.PublishedAt, a.period().TimeZone))
				}
				ui.Row(c).Wrap().Gap(6).Children(func() {
					if spaceButton(c, "Kelola konten / PJ", "edit", false).Disabled(a.offlineMode() || a.medhum.busy || !item.CanEditCalendar && !item.CanAssignDesign).Clicked() {
						a.beginMedhum(&item)
					}
					if item.HumasTaskID != nil && spaceButton(c, "Tugas Humas", "task", false).Clicked() {
						a.OpenNotification(api.Delivery{SourceKind: "task", SourceID: *item.HumasTaskID, SpaceID: &item.SpaceID})
					}
					if item.DesignTaskID != nil && spaceButton(c, "Tugas desain", "task", false).Clicked() {
						a.OpenNotification(api.Delivery{SourceKind: "task", SourceID: *item.DesignTaskID, SpaceID: &item.SpaceID})
					}
				})
			})
		}
		if a.medhum.loaded && len(a.medhum.items) == 0 {
			muted(c, "Belum ada konten pada periode ini.")
		}
	})
}
func (a *App) medhumForm(c *ui.Context) {
	d := a.medhum.draft
	ui.Column(c.Key(d)).FillWidth().Padding(16).Gap(10).Border(1, c.Theme().Border).Radius(10).Children(func() {
		ui.Column(c).FillWidth().Gap(8).Disabled(!d.calendar || a.medhum.busy).Children(func() {
			for _, f := range []struct {
				label string
				value *string
			}{{"Judul konten", &d.title}, {"Jadwal posting · YYYY-MM-DD", &d.date}, {"Jam posting · kosong jika sumber hanya tanggal", &d.clock}, {"Tautan konsep", &d.concept}, {"Kartu Trello · opsional", &d.trello}} {
				ui.TextInput(c.Key(f.label), f.value).Label(f.label).FillWidth()
			}
			ui.TextArea(c, &d.caption).Label("Caption").FillWidth().Height(100)
			ui.Text(c, "PJ Humas").Bold()
			for _, person := range a.medhum.humans {
				person := person
				if spaceButton(c, person.DisplayName, "user", person.UserID == d.humas).Clicked() {
					d.humas = person.UserID
				}
			}
		})
		if d.assignment {
			ui.Text(c, "Tetapkan PJ desain").Bold()
			for _, person := range a.medhum.designers {
				person := person
				if spaceButton(c, person.DisplayName, "user", person.UserID == d.design).Clicked() {
					d.design = person.UserID
				}
			}
		}
		ui.TextInput(c, &d.reason).Label("Alasan perubahan").FillWidth()
		ui.Row(c).Gap(8).Children(func() {
			if spaceButton(c, "Simpan konten", "check", true).Disabled(a.medhum.busy || a.offlineMode()).Clicked() {
				a.saveMedhum()
			}
			if spaceButton(c, "Tutup formulir", "close", false).Disabled(a.medhum.busy).Clicked() {
				a.medhum.draft = nil
			}
		})
	})
}

func (a *App) medhumPolicyForm(c *ui.Context) {
	sp := a.selectedSpace()
	if sp == nil {
		return
	}
	ui.Column(c).FillWidth().Gap(8).Children(func() {
		ui.Text(c, "Kebijakan jam untuk sumber tanggal-saja").Bold()
		muted(c, "Kosong berarti jam belum ditetapkan; aturan berbasis jam tidak dinilai otomatis. Tanggal asli tetap disimpan tanpa jam.")
		canPost, canDesign := false, false
		for _, d := range sp.Divisions {
			if d.Slug == "humas" {
				canPost = a.spacePermissionsForDivision(d.ID, api.CapTasksManage)
			}
			if d.Slug == "mediatek" {
				canDesign = a.spacePermissionsForDivision(d.ID, api.CapTasksManage)
			}
		}
		if ui.TextInput(c, &a.medhum.postingClock).Label("Jam posting · HH:MM").FillWidth().Disabled(!canPost || a.offlineMode()).Changed() {
			a.medhum.policyDirty = true
		}
		if ui.TextInput(c, &a.medhum.designClock).Label("Batas penyerahan desain · HH:MM").FillWidth().Disabled(!canDesign || a.offlineMode()).Changed() {
			a.medhum.policyDirty = true
		}
		if spaceButton(c, "Simpan kebijakan waktu", "check", false).Disabled(a.medhum.busy || a.offlineMode() || !a.medhum.policyDirty).Clicked() {
			a.saveMedhumPolicy(canPost, canDesign)
		}
	})
}
func (a *App) saveMedhumPolicy(post, design bool) {
	cl := a.notificationClient()
	if cl == nil {
		return
	}
	req := api.MedhumPolicyRequest{ExpectedVersion: a.medhum.policy.Version}
	if post {
		v := a.medhum.postingClock
		req.PostingClock = &v
	}
	if design {
		v := a.medhum.designClock
		req.DesignClock = &v
	}
	gen, seq, sid := a.gen, a.medhum.seq, a.medhum.space
	a.medhum.busy = true
	a.async(func() func() {
		policy, err := cl.SetMedhumPolicy(context.Background(), sid, req)
		return func() {
			if gen != a.gen || seq != a.medhum.seq {
				return
			}
			a.medhum.busy = false
			if err != nil {
				a.medhum.err = a.failed(err)
				return
			}
			a.medhum.policy = policy
			a.medhum.policyDirty = false
		}
	})
}
