package ecs

import (
	"fmt"
	"reflect"
)

// Before mutex
// BenchmarkSetup-12    	       4	 303655452 ns/op
// BenchmarkReads-12    	       7	 154781578 ns/op
// BenchmarkEach-12     	      19	  58319213 ns/op
// After Mutex
// BenchmarkSetup-12    	       4	 362666079 ns/op
// BenchmarkReads-12    	       6	 174280272 ns/op
// BenchmarkEach-12     	      20	  62529058 ns/op

// Copy Paste for new types
// type TYPE struct {
// }
// func (t *TYPE) ComponentSet(val interface{}) { *t = val.(TYPE) }

type Id uint32

type Component interface {
	ComponentSet(interface{})
}

type BasicStorage struct {
	list map[Id]interface{}
}

func NewBasicStorage() *BasicStorage {
	return &BasicStorage{
		list: make(map[Id]interface{}),
	}
}

func (s *BasicStorage) Read(id Id) (interface{}, bool) {
	val, ok := s.list[id]
	return val, ok
}

func (s *BasicStorage) Write(id Id, val interface{}) {
	s.list[id] = val
}

func (s *BasicStorage) Delete(id Id) {
	delete(s.list, id)
}

type Engine struct {
	reg map[string]*BasicStorage
	idCounter Id
}

func NewEngine() *Engine {
	return &Engine{
		reg: make(map[string]*BasicStorage),
		idCounter: 0,
	}
}

// TODO - make thread safe!
func (e *Engine) NewId() Id {
	id := e.idCounter
	e.idCounter++
	return id
}

func name(t interface{}) string {
	name := reflect.TypeOf(t).String()
	if name[0] == '*' {
		return name[1:]
	}

	return name
}

func GetStorage(e *Engine, t interface{}) *BasicStorage {
	name := name(t)
	storage, ok := e.reg[name]
	if !ok {
		e.reg[name] = NewBasicStorage()
		storage, _ = e.reg[name]
	}
	return storage
}

func Print(e *Engine, id Id, val interface{}) {
	storage := GetStorage(e, val)
	fmt.Println(storage)
}


func Read(e *Engine, id Id, val Component) bool {
	storage := GetStorage(e, val)
	newVal, ok := storage.Read(id)
	if ok {
		val.ComponentSet(newVal)
	}
	return ok
}

func Write(e *Engine, id Id, val interface{}) {
	storage := GetStorage(e, val)
	storage.Write(id, val)
}

func Delete(engine *Engine, id Id) {
	for _, storage := range engine.reg {
		storage.Delete(id)
	}
}

func Each(engine *Engine, t interface{}, f func(id Id, a interface{})) {
	storage := GetStorage(engine, t)
	for id, a := range storage.list {
		f(id, a)
	}
}
