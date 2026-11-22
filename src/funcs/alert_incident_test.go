package funcs

import (
	"strings"
	"testing"
	"time"

	"github.com/zenlenet/pingmesh/src/g"
)

func resetEvents(t *testing.T) *[]alertMsg {
	t.Helper()
	eventMu.Lock()
	openEvents = map[string]*alertEvent{}
	eventOfLink = map[string]string{}
	eventsReady = true
	eventMu.Unlock()
	sent := &[]alertMsg{}
	old := sendEventMsg
	sendEventMsg = func(m alertMsg) { *sent = append(*sent, m) }
	t.Cleanup(func() { sendEventMsg = old })
	return sent
}

func badLink(from, to string) pendingNotify {
	return pendingNotify{log: g.AlertLog{Fromip: from, Fromname: from, Targetip: to, Targetname: to}, kind: "alert"}
}

func present(keys ...string) map[string]bool {
	m := map[string]bool{}
	for _, k := range keys {
		m[k] = true
	}
	return m
}

func count(sent []alertMsg, kind string) int {
	n := 0
	for _, m := range sent {
		if m.Kind == kind {
			n++
		}
	}
	return n
}

func TestEventOneAlertOneRecovery(t *testing.T) {
	sent := resetEvents(t)
	all := present("A|B", "B|A", "A|C")
	t0 := time.Date(2026, 11, 22, 14, 0, 0, 0, time.Local)
	settleEvents(sweepDelta{
		newBad:  []pendingNotify{badLink("A", "B"), badLink("B", "A"), badLink("A", "C")},
		present: all, now: t0,
		cells: []meshCell{
			{FromIP: "A", FromName: "A", ToIP: "B", ToName: "B"},
			{FromIP: "B", FromName: "B", ToIP: "A", ToName: "A"},
			{FromIP: "A", FromName: "A", ToIP: "C", ToName: "C"},
		},
	})
	if count(*sent, "digest") != 1 {
		t.Fatalf("want 1 alert, got %d", count(*sent, "digest"))
	}
	id := (*sent)[0].Fields[0][1]

	// 先恢复一条，不能单独发恢复
	settleEvents(sweepDelta{recovered: []string{"A|B"}, present: all, now: t0.Add(3 * time.Minute)})
	if count(*sent, "digest_recovery") != 0 {
		t.Fatal("partial recovery must not notify")
	}
	settleEvents(sweepDelta{recovered: []string{"B|A", "A|C"}, present: all, now: t0.Add(5 * time.Minute)})
	if count(*sent, "digest_recovery") != 1 {
		t.Fatalf("want 1 recovery, got %d", count(*sent, "digest_recovery"))
	}
	rec := (*sent)[len(*sent)-1]
	if rec.Fields[0][1] != id {
		t.Fatalf("recovery id %s != alert id %s", rec.Fields[0][1], id)
	}
	if !strings.Contains(rec.Plain, "恢复链路: 3 / 3") {
		t.Fatalf("recovery=%s", rec.Plain)
	}
	// 再来一轮不会重复发恢复
	settleEvents(sweepDelta{recovered: []string{"A|B"}, present: all, now: t0.Add(6 * time.Minute)})
	if count(*sent, "digest_recovery") != 1 {
		t.Fatal("recovery sent twice")
	}
}

func TestRecoveryWithoutAlertIsSilent(t *testing.T) {
	sent := resetEvents(t)
	settleEvents(sweepDelta{recovered: []string{"A|B"}, present: present("A|B"), now: time.Now()})
	if len(*sent) != 0 {
		t.Fatalf("a link that never alerted must not send recovery: %v", *sent)
	}
}

func TestEventClosesWhenLinkRemoved(t *testing.T) {
	sent := resetEvents(t)
	t0 := time.Now()
	settleEvents(sweepDelta{newBad: []pendingNotify{badLink("A", "B")}, present: present("A|B"), now: t0})
	settleEvents(sweepDelta{present: present(), now: t0.Add(time.Minute)})
	if count(*sent, "digest") != 1 || count(*sent, "digest_recovery") != 1 {
		t.Fatalf("alerts=%d recoveries=%d", count(*sent, "digest"), count(*sent, "digest_recovery"))
	}
}

func TestTwoEventsStaySeparate(t *testing.T) {
	sent := resetEvents(t)
	all := present("A|B", "C|D")
	t0 := time.Date(2026, 11, 22, 14, 0, 0, 0, time.Local)
	settleEvents(sweepDelta{newBad: []pendingNotify{badLink("A", "B")}, present: all, now: t0})
	settleEvents(sweepDelta{newBad: []pendingNotify{badLink("C", "D")}, present: all, now: t0.Add(10 * time.Second)})
	settleEvents(sweepDelta{recovered: []string{"A|B", "C|D"}, present: all, now: t0.Add(5 * time.Minute)})
	if count(*sent, "digest") != 2 || count(*sent, "digest_recovery") != 2 {
		t.Fatalf("alerts=%d recoveries=%d", count(*sent, "digest"), count(*sent, "digest_recovery"))
	}
}

func TestEventReminderOncePerInterval(t *testing.T) {
	sent := resetEvents(t)
	all := present("A|B")
	t0 := time.Now()
	settleEvents(sweepDelta{newBad: []pendingNotify{badLink("A", "B")}, present: all, now: t0, remind: 30 * time.Minute})
	settleEvents(sweepDelta{present: all, now: t0.Add(10 * time.Minute), remind: 30 * time.Minute})
	settleEvents(sweepDelta{present: all, now: t0.Add(31 * time.Minute), remind: 30 * time.Minute})
	settleEvents(sweepDelta{present: all, now: t0.Add(32 * time.Minute), remind: 30 * time.Minute})
	if count(*sent, "digest_reminder") != 1 {
		t.Fatalf("reminders=%d", count(*sent, "digest_reminder"))
	}
}
