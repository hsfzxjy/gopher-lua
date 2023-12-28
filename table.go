package lua

const defaultArrayCap = 32
const defaultHashCap = 32

type lValueArraySorter struct {
	L      *LState
	Fn     *LFunction
	Values []LValue
}

func (lv lValueArraySorter) Len() int {
	return len(lv.Values)
}

func (lv lValueArraySorter) Swap(i, j int) {
	lv.Values[i], lv.Values[j] = lv.Values[j], lv.Values[i]
}

func (lv lValueArraySorter) Less(i, j int) bool {
	if lv.Fn != nil {
		lv.L.Push(lv.Fn.AsLValue())
		lv.L.Push(lv.Values[i])
		lv.L.Push(lv.Values[j])
		lv.L.Call(2, 1)
		return LVAsBool(lv.L.reg.Pop())
	}
	return lessThan(lv.L, lv.Values[i], lv.Values[j])
}

func newLTable(acap int, hcap int) *LTable {
	if acap < 0 {
		acap = 0
	}
	if hcap < 0 {
		hcap = 0
	}
	tb := &LTable{}
	tb.Metatable = LNil
	if acap != 0 {
		tb.array = make([]LValue, 0, acap)
	}
	if hcap != 0 {
		tb.strdict = make(map[string]*ltableSlot, hcap)
		tb.slots.Init(hcap)
	} else {
		tb.slots.Init(0)
	}
	return tb
}

// Len returns length of this LTable without using __len.
func (tb *LTable) Len() int {
	if tb.array == nil {
		return 0
	}
	var prev LValue = LNil
	for i := len(tb.array) - 1; i >= 0; i-- {
		v := tb.array[i]
		if prev.EqualsLNil() && !v.EqualsLNil() {
			return i + 1
		}
		prev = v
	}
	return 0
}

// Append appends a given LValue to this LTable.
func (tb *LTable) Append(value LValue) {
	if value.EqualsLNil() {
		return
	}
	if tb.array == nil {
		tb.array = make([]LValue, 0, defaultArrayCap)
	}
	if len(tb.array) == 0 || !tb.array[len(tb.array)-1].EqualsLNil() {
		tb.array = append(tb.array, value)
	} else {
		i := len(tb.array) - 2
		for ; i >= 0; i-- {
			if !tb.array[i].EqualsLNil() {
				break
			}
		}
		tb.array[i+1] = value
	}
}

// Insert inserts a given LValue at position `i` in this table.
func (tb *LTable) Insert(i int, value LValue) {
	if tb.array == nil {
		tb.array = make([]LValue, 0, defaultArrayCap)
	}
	if i > len(tb.array) {
		tb.RawSetInt(i, value)
		return
	}
	if i <= 0 {
		tb.RawSet(LNumber(i).AsLValue(), value)
		return
	}
	i -= 1
	tb.array = append(tb.array, LNil)
	copy(tb.array[i+1:], tb.array[i:])
	tb.array[i] = value
}

// MaxN returns a maximum number key that nil value does not exist before it.
func (tb *LTable) MaxN() int {
	if tb.array == nil {
		return 0
	}
	for i := len(tb.array) - 1; i >= 0; i-- {
		if !tb.array[i].EqualsLNil() {
			return i + 1
		}
	}
	return 0
}

// Remove removes from this table the element at a given position.
func (tb *LTable) Remove(pos int) LValue {
	if tb.array == nil {
		return LNil
	}
	larray := len(tb.array)
	if larray == 0 {
		return LNil
	}
	i := pos - 1
	oldval := LNil
	switch {
	case i >= larray:
		// nothing to do
	case i == larray-1 || i < 0:
		oldval = tb.array[larray-1]
		tb.array = tb.array[:larray-1]
	default:
		oldval = tb.array[i]
		copy(tb.array[i:], tb.array[i+1:])
		tb.array[larray-1] = LValue{}
		tb.array = tb.array[:larray-1]
	}
	return oldval
}

// RawSet sets a given LValue to a given index without the __newindex metamethod.
// It is recommended to use `RawSetString` or `RawSetInt` for performance
// if you already know the given LValue is a string or number.
func (tb *LTable) RawSet(key LValue, value LValue) {
	switch v := key; v.Type() {
	case LTNumber:
		v := v.mustLNumberUnchecked()
		if isArrayKey(v) {
			if tb.array == nil {
				tb.array = make([]LValue, 0, defaultArrayCap)
			}
			index := int(v) - 1
			alen := len(tb.array)
			switch {
			case index == alen:
				tb.array = append(tb.array, value)
			case index > alen:
				for i := 0; i < (index - alen); i++ {
					tb.array = append(tb.array, LNil)
				}
				tb.array = append(tb.array, value)
			case index < alen:
				tb.array[index] = value
			}
			return
		}
	case LTString:
		v := v.mustLStringUnchecked()
		tb.RawSetString(string(v), value)
		return
	}

	tb.RawSetH(key, value)
}

// RawSetInt sets a given LValue at a position `key` without the __newindex metamethod.
func (tb *LTable) RawSetInt(key int, value LValue) {
	if key < 1 || key >= MaxArrayIndex {
		tb.RawSetH(LNumber(key).AsLValue(), value)
		return
	}
	if tb.array == nil {
		tb.array = make([]LValue, 0, 32)
	}
	index := key - 1
	alen := len(tb.array)
	switch {
	case index == alen:
		tb.array = append(tb.array, value)
	case index > alen:
		for i := 0; i < (index - alen); i++ {
			tb.array = append(tb.array, LNil)
		}
		tb.array = append(tb.array, value)
	case index < alen:
		tb.array[index] = value
	}
}

// RawSetString sets a given LValue to a given string index without the __newindex metamethod.
func (tb *LTable) RawSetString(key string, value LValue) {
	if tb.strdict == nil {
		tb.strdict = make(map[string]*ltableSlot, defaultHashCap)
	}

	slot := tb.strdict[key]
	if value.EqualsLNil() {
		delete(tb.strdict, key)
		tb.slots.Release(slot)
	} else if slot != nil {
		slot.value = value
	} else {
		tb.strdict[key] = tb.slots.Put(LString(key).AsLValue(), value)
	}
}

// RawSetH sets a given LValue to a given index without the __newindex metamethod.
func (tb *LTable) RawSetH(key LValue, value LValue) {
	if s, ok := key.AsLString(); ok {
		tb.RawSetString(string(s), value)
		return
	}
	if tb.dict == nil {
		tb.dict = make(map[[2]uintptr]*ltableSlot, len(tb.strdict))
	}

	ckey := key.asComparable()
	slot := tb.dict[ckey]
	if value.EqualsLNil() {
		delete(tb.dict, ckey)
		tb.slots.Release(slot)
	} else if slot != nil {
		slot.value = value
	} else {
		tb.dict[ckey] = tb.slots.Put(key, value)
	}
}

// RawGet returns an LValue associated with a given key without __index metamethod.
func (tb *LTable) RawGet(key LValue) LValue {
	switch v := key; v.Type() {
	case LTNumber:
		v := v.mustLNumberUnchecked()
		if isArrayKey(v) {
			if tb.array == nil {
				return LNil
			}
			index := int(v) - 1
			if index >= len(tb.array) {
				return LNil
			}
			return tb.array[index]
		}
	case LTString:
		v := v.mustLStringUnchecked()
		if tb.strdict == nil {
			return LNil
		}
		if ret, ok := tb.strdict[string(v)]; ok {
			return ret.value
		}
		return LNil
	}
	if tb.dict == nil {
		return LNil
	}
	if v, ok := tb.dict[key.asComparable()]; ok {
		return v.value
	}
	return LNil
}

func (tb *LTable) rawDelete(key LValue) (deleted bool) {
	switch v := key; v.Type() {
	case LTNumber:
		v := v.mustLNumberUnchecked()
		if isArrayKey(v) {
			index := int(v) - 1
			alen := len(tb.array)
			if index < alen {
				deleted = !tb.array[index].EqualsLNil()
				tb.array[index] = LNil
				return deleted
			}
			return false
		}
	case LTString:
		v := v.mustLStringUnchecked()
		return tb.rawDeleteString(string(v))
	}
	return tb.rawDeleteH(key)
}

func (tb *LTable) rawDeleteH(key LValue) (deleted bool) {
	if tb.dict == nil {
		return false
	}
	ckey := key.asComparable()
	if slot, ok := tb.dict[ckey]; ok {
		deleted = !slot.value.EqualsLNil()
		slot.value = LNil
		tb.slots.Release(slot)
		delete(tb.dict, ckey)
		return deleted
	}
	return false
}

func (tb *LTable) rawDeleteString(key string) (deleted bool) {
	if tb.strdict == nil {
		return false
	}
	if slot, ok := tb.strdict[key]; ok {
		deleted = !slot.value.EqualsLNil()
		slot.value = LNil
		tb.slots.Release(slot)
		delete(tb.strdict, key)
		return deleted
	}
	return false
}

func (tb *LTable) rawGetForSet(key LValue) *LValue {
	switch v := key; v.Type() {
	case LTNumber:
		v := v.mustLNumberUnchecked()
		if isArrayKey(v) {
			if tb.array == nil {
				return nil
			}
			index := int(v) - 1
			if index >= len(tb.array) {
				return nil
			}
			return &tb.array[index]
		}
	case LTString:
		v := v.mustLStringUnchecked()
		if tb.strdict == nil {
			return nil
		}
		if ret, ok := tb.strdict[string(v)]; ok {
			return &ret.value
		}
		return nil
	}
	if tb.dict == nil {
		return nil
	}
	if v, ok := tb.dict[key.asComparable()]; ok {
		return &v.value
	}
	return nil
}

// RawGetInt returns an LValue at position `key` without __index metamethod.
func (tb *LTable) RawGetInt(key int) LValue {
	if tb.array == nil {
		return LNil
	}
	index := int(key) - 1
	if index >= len(tb.array) || index < 0 {
		return LNil
	}
	return tb.array[index]
}

// RawGet returns an LValue associated with a given key without __index metamethod.
func (tb *LTable) RawGetH(key LValue) LValue {
	if s, sok := key.AsLString(); sok {
		if tb.strdict == nil {
			return LNil
		}
		if v, vok := tb.strdict[string(s)]; vok {
			return v.value
		}
		return LNil
	}
	if tb.dict == nil {
		return LNil
	}
	if v, ok := tb.dict[key.asComparable()]; ok {
		return v.value
	}
	return LNil
}

// RawGetString returns an LValue associated with a given key without __index metamethod.
func (tb *LTable) RawGetString(key string) LValue {
	if tb.strdict == nil {
		return LNil
	}
	if v, vok := tb.strdict[string(key)]; vok {
		return v.value
	}
	return LNil
}

// ForEach iterates over this table of elements, yielding each in turn to a given function.
func (tb *LTable) ForEach(cb func(LValue, LValue)) {
	if tb.array != nil {
		for i, v := range tb.array {
			if !v.EqualsLNil() {
				cb(LNumber(i+1).AsLValue(), v)
			}
		}
	}
	slot := tb.slots.start
	for slot != nil {
		cb(slot.key, slot.value)
		slot = slot.next
	}
}

// This function is equivalent to lua_next ( http://www.lua.org/manual/5.1/manual.html#lua_next ).
func (tb *LTable) Next(key LValue) (LValue, LValue) {
	init := false
	if key.EqualsLNil() {
		key = LNumber(0).AsLValue()
		init = true
	}

	if init || !key.Equals(LNumber(0).AsLValue()) {
		if kv, ok := key.AsLNumber(); ok && isInteger(kv) && int(kv) >= 0 && kv < LNumber(MaxArrayIndex) {
			index := int(kv)
			if tb.array != nil {
				for ; index < len(tb.array); index++ {
					if v := tb.array[index]; !v.EqualsLNil() {
						return LNumber(index + 1).AsLValue(), v
					}
				}
			}
			if tb.array == nil || index == len(tb.array) {
				if tb.slots.start == nil {
					return LNil, LNil
				}
				return tb.slots.start.key, tb.slots.start.value
			}
		}
	}

	var slot *ltableSlot
	if strKey, ok := key.AsLString(); ok {
		if tb.strdict != nil {
			slot = tb.strdict[string(strKey)]
		}
	} else if tb.dict != nil {
		slot = tb.dict[key.asComparable()]
	}

	if slot == nil || slot.next == nil {
		return LNil, LNil
	}

	return slot.next.key, slot.next.value
}
