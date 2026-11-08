package server

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/himasifunej/himasif-space/internal/api"
	"github.com/himasifunej/himasif-space/internal/id"
)

func taskNumber(t api.Task, space id.ID) int64 { return t.Numbers[space.String()] }

// Tahap 6a: #T numbers count per Space, survive edits, are never reused,
// and a task in two Spaces has a number in each.
func TestTaskNumbersPerSpace(t *testing.T) {
	o := newTaskOrg(t)
	div, medhum := o.spaces["division-mediatek"], o.spaces["medhum"]
	first := o.create(false)
	shared := o.create(true)
	if taskNumber(first, div) != 1 || len(first.Numbers) != 1 {
		t.Fatalf("first task numbers %v", first.Numbers)
	}
	if taskNumber(shared, div) != 2 || taskNumber(shared, medhum) != 1 {
		t.Fatalf("shared task numbers %v", shared.Numbers)
	}
	if got := o.get(shared, o.member); taskNumber(got, div) != 2 || taskNumber(got, medhum) != 1 {
		t.Fatalf("read numbers %v", got.Numbers)
	}

	// An edit that keeps the Spaces keeps the numbers.
	req := o.taskRequest()
	req.Title = "Poster kegiatan (revisi)"
	req.SpaceIDs = []id.ID{medhum}
	req.ExpectedVersion = shared.Version
	var edited api.Task
	if c := o.call("PATCH", taskPath(shared), o.kadiv, req, &edited); c != 200 || taskNumber(edited, div) != 2 || taskNumber(edited, medhum) != 1 {
		t.Fatalf("edit = %d %v", c, edited.Numbers)
	}
	// Leaving Medhum drops that number; coming back takes a new one.
	req.SpaceIDs, req.ExpectedVersion = nil, edited.Version
	if c := o.call("PATCH", taskPath(shared), o.kadiv, req, &edited); c != 200 || len(edited.Numbers) != 1 || taskNumber(edited, div) != 2 {
		t.Fatalf("unlink = %d %v", c, edited.Numbers)
	}
	req.SpaceIDs, req.ExpectedVersion = []id.ID{medhum}, edited.Version
	if c := o.call("PATCH", taskPath(shared), o.kadiv, req, &edited); c != 200 || taskNumber(edited, medhum) != 2 || taskNumber(edited, div) != 2 {
		t.Fatalf("relink = %d %v", c, edited.Numbers)
	}
	// The Space list carries the numbers too.
	var list api.List[api.Task]
	o.call("GET", "/api/spaces/"+div.String()+"/tasks", o.member, nil, &list)
	for _, task := range list.Items {
		if taskNumber(task, div) == 0 {
			t.Fatalf("listed task without number: %+v", task.Numbers)
		}
	}
}

// Concurrent creations in one Space take distinct, gapless numbers.
func TestTaskNumbersConcurrentCreation(t *testing.T) {
	o := newTaskOrg(t)
	div := o.spaces["division-mediatek"]
	const n = 20
	var wg sync.WaitGroup
	var mu sync.Mutex
	var numbers []int64
	codes := make(chan int, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var task api.Task
			code := o.call("POST", "/api/spaces/"+div.String()+"/tasks", o.kadiv, o.taskRequest(), &task)
			codes <- code
			if code == 201 {
				mu.Lock()
				numbers = append(numbers, taskNumber(task, div))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	close(codes)
	for c := range codes {
		if c != 201 {
			t.Fatalf("create = %d", c)
		}
	}
	slices.Sort(numbers)
	for i, got := range numbers {
		if got != int64(i+1) {
			t.Fatalf("numbers %v, want 1..%d", numbers, n)
		}
	}
}

// Migration 0026 numbers existing tasks by creation time, per Space, and
// leaves each counter at its Space's highest number.
func TestTaskNumbersBackfill(t *testing.T) {
	o := newTaskOrg(t)
	ctx := context.Background()
	div, medhum := o.spaces["division-mediatek"], o.spaces["medhum"]
	a := o.create(false)
	b := o.create(true)
	c := o.create(false)
	db := o.st.DB()
	// Created out of order on purpose: c first, then a, then b.
	base := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	for i, tid := range []id.ID{c.ID, a.ID, b.ID} {
		if _, err := db.Exec(ctx, `update tasks set created_at=$2 where id=$1`, tid, base.Add(time.Duration(i)*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	// Back to the schema before 0026, then migrate again.
	if _, err := db.Exec(ctx, `alter table task_spaces drop constraint task_spaces_number;
		alter table task_spaces drop column number;
		drop table space_task_counters;
		delete from schema_migrations where name='0026_task_numbers'`); err != nil {
		t.Fatal(err)
	}
	applied, err := o.st.Migrate(ctx)
	if err != nil || !slices.Contains(applied, "0026_task_numbers") {
		t.Fatalf("migrate applied=%v err=%v", applied, err)
	}
	want := map[id.ID]map[id.ID]int64{c.ID: {div: 1}, a.ID: {div: 2}, b.ID: {div: 3, medhum: 1}}
	for tid, spaces := range want {
		got := o.get(api.Task{ID: tid}, o.kadiv)
		for sid, n := range spaces {
			if taskNumber(got, sid) != n {
				t.Errorf("task %s in %s = %d, want %d (%v)", tid, sid, taskNumber(got, sid), n, got.Numbers)
			}
		}
	}
	var last int64
	if err := db.QueryRow(ctx, `select last_number from space_task_counters where space_id=$1`, div).Scan(&last); err != nil || last != 3 {
		t.Fatalf("counter = %d %v", last, err)
	}
	if next := o.create(false); taskNumber(next, div) != 4 {
		t.Fatalf("next after backfill = %v", next.Numbers)
	}
}
