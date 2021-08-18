package ecs

import (
	"testing"
)

type testData struct {
	value int
}
func (t *testData) ComponentSet(val interface{}) { *t = val.(testData) }

func setup(size int) *Engine {
	engine := NewEngine()

	for i := 0; i < size; i++ {
		id := engine.NewId()
		data := testData{i}
		Write(engine, id, data)
	}
	return engine
}


func BenchmarkSetup(b *testing.B) {
	for i := 0; i < b.N; i++ {
		setup(1e6)
	}
}

func BenchmarkReads(b *testing.B) {
	engine := setup(1e6)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		data := testData{}
		for j := 0; j < 1e6; j++ {
			Read(engine, Id(j), &data)
		}
	}
}

func BenchmarkEach(b *testing.B) {
	engine := setup(1e6)
	b.ResetTimer()

	data := testData{}
	for i := 0; i < b.N; i++ {
		Each(engine, testData{}, func(id Id, a interface{}) {
			data = a.(testData)
		})
	}
	data.value = data.value + data.value
}
