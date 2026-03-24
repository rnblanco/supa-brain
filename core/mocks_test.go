package core

import (
	"context"
)

type mockEmbedder struct {
	vector []float32
	err    error
}

func (m *mockEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	if m.err != nil {
		return nil, m.err
	}
	v := make([]float32, len(m.vector))
	copy(v, m.vector)
	return v, nil
}

type mockStore struct {
	inserted  []Memory
	upserted  []Memory
	sessions  []Session
	returnID  int64
	returnMem *Memory
	err       error
}

func (m *mockStore) Insert(_ context.Context, mem Memory) (int64, error) {
	m.inserted = append(m.inserted, mem)
	return m.returnID, m.err
}
func (m *mockStore) Upsert(_ context.Context, mem Memory) (int64, error) {
	m.upserted = append(m.upserted, mem)
	return m.returnID, m.err
}
func (m *mockStore) Search(_ context.Context, _ SearchQuery) ([]MemoryResult, error) {
	return nil, m.err
}
func (m *mockStore) GetByID(_ context.Context, _ int64) (*Memory, error) {
	return m.returnMem, m.err
}
func (m *mockStore) Delete(_ context.Context, _, _ string) error { return m.err }
func (m *mockStore) SaveSession(_ context.Context, s Session, _ []float32) error {
	m.sessions = append(m.sessions, s)
	return m.err
}
func (m *mockStore) Export(_ context.Context, _ ExportFilter) ([]Memory, error) {
	return nil, m.err
}
