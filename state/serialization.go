package state

import "encoding/json"

func SerializeSnapshot(snapshot Snapshot) ([]byte, error) {
	return json.MarshalIndent(snapshot, "", "  ")
}

func DeserializeSnapshot(data []byte) (Snapshot, error) {
	var snapshot Snapshot

	err := json.Unmarshal(data, &snapshot)
	return snapshot, err
}
