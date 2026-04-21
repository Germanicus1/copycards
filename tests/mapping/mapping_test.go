package mapping

import (
	"os"
	"testing"

	"copycards/internal/mapping"
)

func TestMappingLoadSave(t *testing.T) {
	tmpfile, _ := os.CreateTemp("", "mapping*.json")
	defer os.Remove(tmpfile.Name())
	tmpfile.Close()

	m := &mapping.Mapping{
		From:            "src",
		To:              "dst",
		SrcBoardID:      "board1",
		DstBoardID:      "board2",
		Users:           make(map[string]string),
		Tickets:         make(map[string]string),
		TicketTypes:     make(map[string]string),
		CustomFields:    make(map[string]string),
		Bins:            make(map[string]string),
		UserGroups:      make(map[string]string),
		Comments:        make(map[string]string),
		Attachments:     make(map[string]string),
	}

	m.RecordUser("user1", "user2")
	m.RecordTicket("ticket1", "ticket2")

	if err := m.Save(tmpfile.Name()); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	m2, err := mapping.Load(tmpfile.Name())
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if m2.From != "src" || m2.GetTicketDst("ticket1") != "ticket2" {
		t.Errorf("Mapping mismatch after load")
	}
}

func TestMappingLoadNonexistent(t *testing.T) {
	m, err := mapping.Load("/nonexistent/path/mapping.json")
	if err != nil {
		t.Fatalf("Load nonexistent should return empty mapping, got: %v", err)
	}
	if m == nil || len(m.Tickets) != 0 {
		t.Errorf("Expected empty mapping, got: %v", m)
	}
}
