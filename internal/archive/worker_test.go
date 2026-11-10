package archive

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ReanSn0w/justkb/internal/maintenance"
)

func TestWorkerBoundaryProjectFirstAndRestoreGrace(t *testing.T) {
	db := fixture(t)
	ctx := context.Background()
	s := NewService(db)
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	if _, err := db.Exec(`UPDATE cards SET column_entered_at=$1; UPDATE epics SET completed_since=$1`, stamp(now.Add(-72*time.Hour))); err != nil {
		t.Fatal(err)
	}
	p, err := s.Policy(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p.CardsEnabled = true
	p.ProjectsEnabled = true
	if _, err = s.SavePolicy(ctx, p, p.Version); err != nil {
		t.Fatal(err)
	}
	gate := &maintenance.Gate{}
	now = now.Add(-time.Second)
	result, err := s.RunOnce(ctx, gate)
	if err != nil || result != (PassResult{}) {
		t.Fatalf("before boundary: %+v %v", result, err)
	}
	now = now.Add(time.Second)
	result, err = s.RunOnce(ctx, gate)
	if err != nil || result.Projects != 1 || result.Cards != 0 {
		t.Fatalf("at boundary: %+v %v", result, err)
	}
	result, err = NewService(db).RunOnce(ctx, gate)
	if err != nil || result != (PassResult{}) {
		t.Fatalf("restart: %+v %v", result, err)
	}
	if err := write(t, db, func(tx *sql.Tx) error { return Project(ctx, tx, Actor{UserID: "u"}, "p", 2, false, "with_cards", now) }); err != nil {
		t.Fatal(err)
	}
	result, err = s.RunOnce(ctx, gate)
	if err != nil || result != (PassResult{}) {
		t.Fatalf("restore grace: %+v %v", result, err)
	}
}
func TestWorkerReReadsPolicyAfterMaintenance(t *testing.T) {
	db := fixture(t)
	s := NewService(db)
	gate := &maintenance.Gate{}
	ctx := context.Background()
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- gate.Exclusive(func() error {
			close(entered)
			<-release
			_, err := db.Exec(`UPDATE archive_policy SET cards_enabled=0`)
			return err
		})
	}()
	<-entered
	workerDone := make(chan error, 1)
	go func() { _, err := s.RunOnce(ctx, gate); workerDone <- err }()
	select {
	case err := <-workerDone:
		t.Fatalf("worker passed exclusive gate: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := <-workerDone; err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM archive_events`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("stale policy: %d %v", n, err)
	}
}

func TestWorkerIndependentDurationsAndIncompleteProjects(t *testing.T) {
	db := fixture(t)
	ctx := context.Background()
	s := NewService(db)
	now := time.Now().UTC()
	s.now = func() time.Time { return now }
	gate := &maintenance.Gate{}
	if err := write(t, db, func(tx *sql.Tx) error { return RefreshProjects(ctx, tx, now.Add(-4*24*time.Hour), "p") }); err != nil {
		t.Fatal(err)
	}
	p, err := s.Policy(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p.CardsEnabled = true
	p.ProjectsEnabled = true
	p.CardsDays = 3
	p.ProjectsDays = 7
	if _, err = s.SavePolicy(ctx, p, p.Version); err != nil {
		t.Fatal(err)
	}
	result, err := s.RunOnce(ctx, gate)
	if err != nil || result.Cards != 3 || result.Projects != 0 {
		t.Fatalf("independent durations: %+v %v", result, err)
	}
	now = now.Add(3 * 24 * time.Hour)
	result, err = s.RunOnce(ctx, gate)
	if err != nil || result.Projects != 1 {
		t.Fatalf("archived cards still complete project: %+v %v", result, err)
	}
}
func TestRoundedProgressNeverArchivesIncompleteProject(t *testing.T) {
	db := fixture(t)
	ctx := context.Background()
	s := NewService(db)
	now := time.Now()
	s.now = func() time.Time { return now }
	_, err := db.Exec(`DELETE FROM cards;WITH RECURSIVE n(x) AS (VALUES(1) UNION ALL SELECT x+1 FROM n WHERE x<201) INSERT INTO cards(id,number,title,column_id,epic_id,position,created_by,column_entered_at) SELECT 'c'||x,x,'Task',CASE WHEN x=201 THEN (SELECT id FROM board_columns ORDER BY position LIMIT 1) ELSE (SELECT done_column_id FROM archive_policy) END,'p',x*100,'u','2020-01-01' FROM n;UPDATE epics SET completed_since='2020-01-01';UPDATE archive_policy SET projects_enabled=1;INSERT INTO epics(id,name,color,completed_since) VALUES('empty','Empty','#112233','2020-01-01');`)
	if err != nil {
		t.Fatal(err)
	}
	result, err := s.RunOnce(ctx, &maintenance.Gate{})
	if err != nil || result.Projects != 0 {
		t.Fatalf("rounded/empty project: %+v %v", result, err)
	}
}
