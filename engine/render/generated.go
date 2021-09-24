package render

type SpriteList []Sprite
func (t *SpriteList) ComponentSet(val interface{}) { *t = *val.(*SpriteList) }
func (t *SpriteList) InternalRead(index int, val interface{}) { *val.(*Sprite) = (*t)[index] }
func (t *SpriteList) InternalWrite(index int, val interface{}) { (*t)[index] = val.(Sprite) }
func (t *SpriteList) InternalAppend(val interface{}) { (*t) = append((*t), val.(Sprite)) }
func (t *SpriteList) InternalPointer(index int) interface{} { return &(*t)[index] }
func (t *SpriteList) InternalReadVal(index int) interface{} { return (*t)[index] }
func (t *SpriteList) Delete(index int) {
	lastVal := (*t)[len(*t)-1]
	(*t)[index] = lastVal
	(*t) = (*t)[:len(*t)-1]
}
func (t *SpriteList) Len() int { return len(*t) }

type KeybindsList []Keybinds
func (t *KeybindsList) ComponentSet(val interface{}) { *t = *val.(*KeybindsList) }
func (t *KeybindsList) InternalRead(index int, val interface{}) { *val.(*Keybinds) = (*t)[index] }
func (t *KeybindsList) InternalWrite(index int, val interface{}) { (*t)[index] = val.(Keybinds) }
func (t *KeybindsList) InternalAppend(val interface{}) { (*t) = append((*t), val.(Keybinds)) }
func (t *KeybindsList) InternalPointer(index int) interface{} { return &(*t)[index] }
func (t *KeybindsList) InternalReadVal(index int) interface{} { return (*t)[index] }
func (t *KeybindsList) Delete(index int) {
	lastVal := (*t)[len(*t)-1]
	(*t)[index] = lastVal
	(*t) = (*t)[:len(*t)-1]
}
func (t *KeybindsList) Len() int { return len(*t) }
