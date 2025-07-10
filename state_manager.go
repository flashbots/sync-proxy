package main

type SlotStage int

const (
	FCUOpen SlotStage = iota
	FCUClose
	Payload
	Unknown
)

func (s SlotStage) String() string {
	var str string
	switch s {
	case FCUOpen:
		str = "Forkchoice_update_Open"
	case FCUClose:
		str = "Forkchoice_update_Close"
	case Payload:
		str = "New_Payload"
	case Unknown:
		str = "Unknown"
	}

	return str
}

type StateManager struct {
	entry *BeaconEntry
	stage SlotStage
}

func InitializeStateManager(stage SlotStage, entry *BeaconEntry) *StateManager {
	return &StateManager{
		entry: entry,
		stage: stage,
	}
}

func (m *StateManager) CurrentSlotStage() SlotStage {
	return m.stage
}

func (m *StateManager) NextSlotStage() {
	if m.stage == FCUClose {
		m.stage = FCUOpen
		return
	}

	m.stage = FCUClose
}

func (m *StateManager) Entry() *BeaconEntry {
	return m.entry
}
