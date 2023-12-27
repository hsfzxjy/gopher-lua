package lua

import (
	"strconv"
	"unsafe"
)

type CustomDataHelper[T any] struct {
	entry *customDataEntry
}

func (h *CustomDataHelper[T]) AsLValue(raw *T) LValue {
	return LValue{unsafe.Pointer(raw), uintptr(h.entry.typ) + maxUintptr}
}

func (h *CustomDataHelper[T]) As(lv LValue) (*T, bool) {
	if lv.dataptr == ltSentinelNumber || lv.data != uintptr(h.entry.typ)+maxUintptr {
		return nil, false
	}
	return (*T)(lv.dataptr), true
}

func (h *CustomDataHelper[T]) Must(lv LValue) *T {
	if v, ok := h.As(lv); ok {
		return v
	}
	panic("not a custom data with type " + strconv.Itoa(int(h.entry.typ)))
}

type customDataEntry struct {
	typInfo   unsafe.Pointer
	metatable *LTable
	typ       LValueType
}

func (c *customDataEntry) PackAny(data unsafe.Pointer) any {
	var a = struct{ typ, data unsafe.Pointer }{c.typInfo, data}
	return *(*any)(unsafe.Pointer(&a))
}

type customDataRegistry struct {
	entries []*customDataEntry
	nextTyp LValueType
}

var cdr = &customDataRegistry{
	entries: make([]*customDataEntry, 0, 32),
	nextTyp: LTUnknown + 1,
}

func (cd *customDataRegistry) MaxTyp() LValueType {
	return cd.nextTyp
}

func (cd *customDataRegistry) Entry(typ LValueType) (*customDataEntry, bool) {
	if typ >= cd.nextTyp {
		return nil, false
	}
	if typ <= LTUnknown {
		return nil, false
	}
	idx := int(typ - LTUnknown - 1)
	return cd.entries[idx], true
}

func RegisterCustomData[T any](metatable *LTable) *CustomDataHelper[T] {
	var dummy T
	var i = any(dummy)
	var typInfo = (*struct{ typ, _ unsafe.Pointer })(unsafe.Pointer(&i)).typ
	entry := &customDataEntry{
		typInfo,
		metatable,
		cdr.nextTyp,
	}
	cdr.entries = append(cdr.entries, entry)
	cdr.nextTyp++
	return &CustomDataHelper[T]{entry}
}
