package appui

import (
	"context"
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
	err, filter, tab          string
	month, day                time.Time
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
