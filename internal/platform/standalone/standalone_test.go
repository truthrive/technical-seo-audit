package standalone

import (
	"context"
	"testing"
)

func TestRunStateIsTerminal(t *testing.T) {
	terminalStates := []string{
		StateCompleted,
		StateCancelled,
		StateStopped,
		StateFailed,
		StateInterrupted,
	}
	for _, s := range terminalStates {
		if !IsTerminal(s) {
			t.Errorf("expected state %q to be terminal", s)
		}
	}

	nonTerminalStates := []string{
		StateRunning,
		StatePaused,
		StateDeleting,
		"unknown-state",
	}
	for _, s := range nonTerminalStates {
		if IsTerminal(s) {
			t.Errorf("expected state %q NOT to be terminal", s)
		}
	}
}

func TestCancellable(t *testing.T) {
	c := NewCancellable(context.Background())
	select {
	case <-c.Done():
		t.Fatal("cancellable should not be done initially")
	default:
	}

	c.Cancel()

	select {
	case <-c.Done():
		if err := c.Err(); err != context.Canceled {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	default:
		t.Fatal("cancellable should be done after Cancel()")
	}
}

func TestEventCollector(t *testing.T) {
	col := NewEventCollector()
	if col.Len() != 0 {
		t.Fatalf("expected empty collector, got len %d", col.Len())
	}

	col.Emit("test.event", map[string]string{"key": "value"})
	col.Emit("test.progress", 42)

	if col.Len() != 2 {
		t.Fatalf("expected len 2, got %d", col.Len())
	}

	events := col.Events()
	if len(events) != 2 {
		t.Fatalf("expected 2 events in copy, got %d", len(events))
	}
	if events[0].Name != "test.event" || events[1].Name != "test.progress" {
		t.Errorf("unexpected event names: %+v", events)
	}

	col.Clear()
	if col.Len() != 0 {
		t.Fatalf("expected 0 after Clear(), got %d", col.Len())
	}
}

func TestOpenDB(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("OpenDB(:memory:) failed: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY, name TEXT);")
	if err != nil {
		t.Fatalf("create table failed: %v", err)
	}

	_, err = db.Exec("INSERT INTO test_table (name) VALUES ('sitecrawl');")
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}

	var name string
	err = db.QueryRow("SELECT name FROM test_table WHERE id = 1;").Scan(&name)
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}
	if name != "sitecrawl" {
		t.Errorf("expected 'sitecrawl', got %q", name)
	}
}
