package mmo

type BodyList []Body
func (t *BodyList) ComponentSet(val interface{}) { *t = *val.(*BodyList) }
func (t *BodyList) InternalRead(index int, val interface{}) { *val.(*Body) = (*t)[index] }
func (t *BodyList) InternalWrite(index int, val interface{}) { (*t)[index] = *val.(*Body) }
func (t *BodyList) InternalAppend(val interface{}) { (*t) = append((*t), val.(Body)) }
func (t *BodyList) InternalPointer(index int) interface{} { return &(*t)[index] }
func (t *BodyList) InternalReadVal(index int) interface{} { return (*t)[index] }
func (t *BodyList) Delete(index int) {
	lastVal := (*t)[len(*t)-1]
	(*t)[index] = lastVal
	(*t) = (*t)[:len(*t)-1]
}
func (t *BodyList) Len() int { return len(*t) }

type ClientOwnedList []ClientOwned
func (t *ClientOwnedList) ComponentSet(val interface{}) { *t = *val.(*ClientOwnedList) }
func (t *ClientOwnedList) InternalRead(index int, val interface{}) { *val.(*ClientOwned) = (*t)[index] }
func (t *ClientOwnedList) InternalWrite(index int, val interface{}) { (*t)[index] = *val.(*ClientOwned) }
func (t *ClientOwnedList) InternalAppend(val interface{}) { (*t) = append((*t), val.(ClientOwned)) }
func (t *ClientOwnedList) InternalPointer(index int) interface{} { return &(*t)[index] }
func (t *ClientOwnedList) InternalReadVal(index int) interface{} { return (*t)[index] }
func (t *ClientOwnedList) Delete(index int) {
	lastVal := (*t)[len(*t)-1]
	(*t)[index] = lastVal
	(*t) = (*t)[:len(*t)-1]
}
func (t *ClientOwnedList) Len() int { return len(*t) }

type UserList []User
func (t *UserList) ComponentSet(val interface{}) { *t = *val.(*UserList) }
func (t *UserList) InternalRead(index int, val interface{}) { *val.(*User) = (*t)[index] }
func (t *UserList) InternalWrite(index int, val interface{}) { (*t)[index] = *val.(*User) }
func (t *UserList) InternalAppend(val interface{}) { (*t) = append((*t), val.(User)) }
func (t *UserList) InternalPointer(index int) interface{} { return &(*t)[index] }
func (t *UserList) InternalReadVal(index int) interface{} { return (*t)[index] }
func (t *UserList) Delete(index int) {
	lastVal := (*t)[len(*t)-1]
	(*t)[index] = lastVal
	(*t) = (*t)[:len(*t)-1]
}
func (t *UserList) Len() int { return len(*t) }
