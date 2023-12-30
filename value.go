package lua

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"unsafe"
)

type LValueType int

const (
	LTNil LValueType = iota
	LTBool
	LTNumber
	LTString
	LTFunction
	LTUserData
	LTThread
	LTTable
	LTChannel
	LTUnknown
)

var lValueNames = [9]string{"nil", "boolean", "number", "string", "function", "userdata", "thread", "table", "channel"}

func (vt LValueType) String() string {
	return lValueNames[int(vt)]
}

type LValue struct {
	_       [0]func()
	dataptr unsafe.Pointer
	data    uintptr
}

func (lv LValue) asComparable() [2]uintptr {
	return *(*[2]uintptr)(unsafe.Pointer(&lv))
	// return [2]uintptr{uintptr(lv.dataptr), lv.data}
}

var (
	ltSentinelNumber byte
	ltSentinelTrue   byte
	ltSentinelFalse  byte
)

const maxUintptr = ^uintptr(0)>>1 + 1

func AnyEqual(v1, v2 any) bool {
	v11, ok1 := v1.(LValue)
	v22, ok2 := v2.(LValue)
	if !ok1 && !ok2 {
		return v1 == v2
	}
	if ok1 && ok2 {
		return v11.Equals(v22)
	}
	if ok1 {
		return v11.AsAny() == v2
	}
	return v1 == v22.AsAny()
}

func AnyNormalize(x any) any {
	if x, ok := x.(LValue); ok {
		return x.AsAny()
	}
	return x
}

func AnysNormalize(x []any) []any {
	for i, v := range x {
		x[i] = AnyNormalize(v)
	}
	return x
}

func (lv LValue) IsEmpty() bool {
	return lv.dataptr == nil && lv.data == 0
}

func (lv LValue) Equals(other LValue) bool {
	if v2, ok2 := other.AsLString(); !ok2 {
		return lv.dataptr == other.dataptr && lv.data == other.data
	} else if v1, ok1 := lv.AsLString(); !ok1 {
		return false
	} else {
		return v1 == v2
	}
	// if v1, ok1 := lv.AsLString(); !ok1 {
	// 	return lv.dataptr == other.dataptr && lv.data == other.data
	// } else if v2, ok2 := other.AsLString(); !ok2 {
	// 	return false
	// } else {
	// 	return v1 == v2
	// }
}

func (lv LValue) EqualsLNil() bool {
	return lv.IsEmpty()
}

func (lv LValue) String() string {
	switch lv.Type() {
	case LTNil:
		v, _ := lv.AsLNil()
		return v.String()
	case LTBool:
		v, _ := lv.AsLBool()
		return v.String()
	case LTNumber:
		v, _ := lv.AsLNumber()
		return v.String()
	case LTString:
		v, _ := lv.AsLString()
		return v.String()
	case LTTable:
		v, _ := lv.AsLTable()
		return v.String()
	case LTFunction:
		v, _ := lv.AsLFunction()
		return v.String()
	case LTUserData:
		v, _ := lv.AsLUserData()
		return v.String()
	case LTThread:
		v, _ := lv.AsLThread()
		return v.String()
	case LTChannel:
		v, _ := lv.AsLChannel()
		return v.String()
	default:
		panic("unreachable")
	}
}

func (lv LValue) AsAny() any {
	switch t := lv.Type(); t {
	case LTNil:
		v, _ := lv.AsLNil()
		return v
	case LTBool:
		v, _ := lv.AsLBool()
		return v
	case LTNumber:
		v, _ := lv.AsLNumber()
		return v
	case LTString:
		v, _ := lv.AsLString()
		return v
	case LTTable:
		v, _ := lv.AsLTable()
		return v
	case LTFunction:
		v, _ := lv.AsLFunction()
		return v
	case LTUserData:
		v, _ := lv.AsLUserData()
		return v
	case LTThread:
		v, _ := lv.AsLThread()
		return v
	case LTChannel:
		v, _ := lv.AsLChannel()
		return v
	case LTUnknown:
		return nil
	default:
		if entry, ok := cdr.Entry(t); ok {
			return entry.PackAny(lv.dataptr)
		}
		panic("unreachable")
	}
}

func (lv LValue) AsLValue() LValue { return lv }

func (lv LValue) Type() LValueType {
	if lv.IsEmpty() {
		return LTNil
	}
	if lv.dataptr == unsafe.Pointer(&ltSentinelNumber) {
		return LTNumber
	}
	if lv.data&maxUintptr == 0 {
		return LTString
	}
	typ := lv.data - maxUintptr
	if typ < uintptr(LTUnknown) {
		return LValueType(typ)
	}
	if _, ok := cdr.Entry(LValueType(typ)); ok {
		return LValueType(typ)
	}
	return LTUnknown
}

func (lv LValue) MustLNil() *LNilType {
	if v, ok := lv.AsLNil(); ok {
		return v
	}
	panic("not nil")
}

func (lv LValue) AsLNil() (*LNilType, bool) {
	if lv.dataptr == nil && lv.data == 0 {
		return LNilValue, true
	}
	return nil, false
}

func (lv LValue) MustLBool() LBool {
	if v, ok := lv.AsLBool(); ok {
		return v
	}
	panic("not bool")
}

func (lv LValue) AsLBool() (LBool, bool) {
	if lv.dataptr == unsafe.Pointer(&ltSentinelTrue) {
		return LTrue, true
	}
	if lv.dataptr == unsafe.Pointer(&ltSentinelFalse) {
		return LFalse, true
	}
	return LFalse, false
}

func (lv LValue) mustLNumberUnchecked() LNumber {
	return LNumber(math.Float64frombits(uint64(lv.data)))
}

func (lv LValue) MustLNumber() LNumber {
	if v, ok := lv.AsLNumber(); ok {
		return v
	}
	panic("not number")
}

func (lv LValue) AsLNumber() (LNumber, bool) {
	if lv.dataptr == unsafe.Pointer(&ltSentinelNumber) {
		return LNumber(math.Float64frombits(uint64(lv.data))), true
	}
	return LNumber(0), false
}

func (lv LValue) mustLStringUnchecked() LString {
	return LString(unsafe.String((*byte)(lv.dataptr), int(lv.data)))
}

func (lv LValue) MustLString() LString {
	if v, ok := lv.AsLString(); ok {
		return v
	}
	panic("not string")
}

func (lv LValue) AsLString() (LString, bool) {
	if lv.dataptr == unsafe.Pointer(&ltSentinelNumber) || lv.IsEmpty() {
		return "", false
	}
	if lv.data&maxUintptr == 0 {
		str := unsafe.String((*byte)(lv.dataptr), int(lv.data))
		return LString(str), true
	}
	return "", false
}

func (lv LValue) MustLTable() *LTable {
	if v, ok := lv.AsLTable(); ok {
		return v
	}
	panic("not table")
}

func (lv LValue) AsLTable() (*LTable, bool) {
	if lv.dataptr == unsafe.Pointer(&ltSentinelNumber) || lv.data != maxUintptr+uintptr(LTTable) {
		return nil, false
	}
	return (*LTable)(lv.dataptr), true
}

func (lv LValue) MustLFunction() *LFunction {
	if v, ok := lv.AsLFunction(); ok {
		return v
	}
	panic("not function")
}

func (lv LValue) AsLFunction() (*LFunction, bool) {
	if lv.dataptr == unsafe.Pointer(&ltSentinelNumber) || lv.data != maxUintptr+uintptr(LTFunction) {
		return nil, false
	}
	return (*LFunction)(lv.dataptr), true
}

func (lv LValue) MustLUserData() *LUserData {
	if v, ok := lv.AsLUserData(); ok {
		return v
	}
	panic("not userdata")
}

func (lv LValue) AsLUserData() (*LUserData, bool) {
	if lv.dataptr == unsafe.Pointer(&ltSentinelNumber) || lv.data != maxUintptr+uintptr(LTUserData) {
		return nil, false
	}
	return (*LUserData)(lv.dataptr), true
}

func (lv LValue) MustLChannel() LChannel {
	if v, ok := lv.AsLChannel(); ok {
		return v
	}
	panic("not channel")
}

func (lv LValue) AsLChannel() (LChannel, bool) {
	if lv.dataptr == unsafe.Pointer(&ltSentinelNumber) || lv.data != maxUintptr+uintptr(LTChannel) {
		return nil, false
	}
	return *(*LChannel)(unsafe.Pointer(&lv.dataptr)), true
}

func (lv LValue) MustLThread() *LState {
	if v, ok := lv.AsLThread(); ok {
		return v
	}
	panic("not thread")
}

func (lv LValue) AsLThread() (*LState, bool) {
	if lv.dataptr == unsafe.Pointer(&ltSentinelNumber) || lv.data != maxUintptr+uintptr(LTThread) {
		return nil, false
	}
	return (*LState)(lv.dataptr), true
}

func (lv LValue) MustLState() *LState {
	return lv.MustLThread()
}

func (lv LValue) AsLState() (*LState, bool) {
	return lv.AsLThread()
}

// LVIsFalse returns true if a given LValue is a nil or false otherwise false.
func LVIsFalse(v LValue) bool { return v.EqualsLNil() || v.dataptr == unsafe.Pointer(&ltSentinelFalse) }

// LVIsFalse returns false if a given LValue is a nil or false otherwise true.
func LVAsBool(v LValue) bool { return !v.EqualsLNil() && v.dataptr != unsafe.Pointer(&ltSentinelFalse) }

// LVAsString returns string representation of a given LValue
// if the LValue is a string or number, otherwise an empty string.
func LVAsString(v LValue) string {
	if n, ok := v.AsLNumber(); ok {
		return n.String()
	} else if s, ok := v.AsLString(); ok {
		return string(s)
	}
	return ""
}

// LVCanConvToString returns true if a given LValue is a string or number
// otherwise false.
func LVCanConvToString(v LValue) bool {
	typ := v.Type()
	return typ == LTString || typ == LTNumber
}

// LVAsNumber tries to convert a given LValue to a number.
func LVAsNumber(v LValue) LNumber {
	if n, ok := v.AsLNumber(); ok {
		return n
	} else if s, ok := v.AsLString(); ok {
		if num, err := parseNumber(string(s)); err == nil {
			return num
		}
	}
	return LNumber(0)
}

type LNilType struct{}

func (nl *LNilType) String() string   { return "nil" }
func (nl *LNilType) Type() LValueType { return LTNil }
func (nl *LNilType) AsLValue() LValue { return LValue{} }

var LNilValue = new(LNilType)
var LNil = LValue{}

type LBool bool

func (bl LBool) String() string {
	if bool(bl) {
		return "true"
	}
	return "false"
}
func (bl LBool) Type() LValueType { return LTBool }
func (bl LBool) AsLValue() LValue {
	if bool(bl) {
		return LValue{dataptr: unsafe.Pointer(&ltSentinelTrue), data: maxUintptr + uintptr(LTBool)}
	} else {
		return LValue{dataptr: unsafe.Pointer(&ltSentinelFalse), data: maxUintptr + uintptr(LTBool)}
	}
}

var LTrue = LBool(true)
var LFalse = LBool(false)

type LString string

var emptyStringPtr = new(byte)

func (st LString) String() string   { return string(st) }
func (st LString) Type() LValueType { return LTString }
func (st LString) AsLValue() LValue {
	if st == "" {
		return LValue{dataptr: unsafe.Pointer(emptyStringPtr), data: 0}
	}
	return LValue{dataptr: unsafe.Pointer(unsafe.StringData(string(st))), data: uintptr(len(st))}
}

// fmt.Formatter interface
func (st LString) Format(f fmt.State, c rune) {
	switch c {
	case 'd', 'i':
		if nm, err := parseNumber(string(st)); err != nil {
			defaultFormat(nm, f, 'd')
		} else {
			defaultFormat(string(st), f, 's')
		}
	default:
		defaultFormat(string(st), f, c)
	}
}

func (nm LNumber) String() string {
	if isInteger(nm) {
		return strconv.FormatInt(int64(nm), 10)
	}
	return strconv.FormatFloat(float64(nm), 'g', -1, 64)
}

func (nm LNumber) Type() LValueType { return LTNumber }
func (nm LNumber) AsLValue() LValue {
	return LValue{dataptr: unsafe.Pointer(&ltSentinelNumber), data: uintptr(math.Float64bits(float64(nm)))}
}

// fmt.Formatter interface
func (nm LNumber) Format(f fmt.State, c rune) {
	switch c {
	case 'q', 's':
		defaultFormat(nm.String(), f, c)
	case 'b', 'c', 'd', 'o', 'x', 'X', 'U':
		defaultFormat(int64(nm), f, c)
	case 'e', 'E', 'f', 'F', 'g', 'G':
		defaultFormat(float64(nm), f, c)
	case 'i':
		defaultFormat(int64(nm), f, 'd')
	default:
		if isInteger(nm) {
			defaultFormat(int64(nm), f, c)
		} else {
			defaultFormat(float64(nm), f, c)
		}
	}
}

type ltableSlot struct {
	prev, next *ltableSlot
	key, value LValue
}

func (s *ltableSlot) isHead() bool { return s.prev == nil }
func (s *ltableSlot) isTail() bool { return s.next == nil }

const blockSize = 32

type ltableSlotBlock struct {
	ntouched int
	data     [blockSize]ltableSlot
}

func (block *ltableSlotBlock) isFull() bool {
	return block.ntouched == blockSize
}

func (block *ltableSlotBlock) alloc() (*ltableSlot, int) {
	if block.ntouched >= len(block.data) {
		return nil, -1
	}
	slot := &block.data[block.ntouched]
	block.ntouched++
	return slot, block.ntouched - 1
}

type ltableSlots struct {
	blocks       []*ltableSlotBlock
	lastBlockIdx int
	freeHead     *ltableSlot
	start, end   *ltableSlot
}

func (slots *ltableSlots) grow(size int) {
	if size <= 0 {
		return
	}
	moreBlocks := ((size - 1) / blockSize) + 1 - len(slots.blocks)
	if moreBlocks <= 0 {
		return
	}
	data := make([]ltableSlotBlock, moreBlocks)
	for i := range data {
		slots.blocks = append(slots.blocks, &data[i])
	}
}

func (slots *ltableSlots) Init(size int) {
	slots.grow(size)
}

func (slots *ltableSlots) Release(slot *ltableSlot) {
	if slot == nil {
		return
	}
	if slot.isHead() && slot.isTail() {
		slots.start = nil
		slots.end = nil
	} else if slot.isHead() {
		slot.next.prev = nil
		slots.start = slot.next
	} else if slot.isTail() {
		slot.prev.next = nil
		slots.end = slot.prev
	} else {
		prev, next := slot.prev, slot.next
		prev.next = next
		next.prev = prev
	}
	*slot = ltableSlot{}
	slot.key = LValue{}
	slot.value = LValue{}
	slot.prev = nil
	slot.next = slots.freeHead
	slots.freeHead = slot
}

func (slots *ltableSlots) Put(key, value LValue) *ltableSlot {
	var availBlock *ltableSlotBlock
GET_AVAIL_BLOCK:
	for slots.lastBlockIdx < len(slots.blocks) {
		block := slots.blocks[slots.lastBlockIdx]
		if !block.isFull() {
			availBlock = block
			break
		}
		slots.lastBlockIdx++
	}
	if availBlock == nil && slots.freeHead == nil {
		slots.grow(max(len(slots.blocks)*blockSize*3/2, 1))
		goto GET_AVAIL_BLOCK
	}
	var slot *ltableSlot
	if availBlock != nil {
		slot, _ = availBlock.alloc()
	} else {
		slot = slots.freeHead
		slots.freeHead = slot.next
		slot.prev = nil
		slot.next = nil
	}
	slot.key = key
	slot.value = value
	if slots.start == nil {
		slots.start = slot
	}
	if slots.end != nil {
		slot.prev = slots.end
		slots.end.next = slot
	}
	slots.end = slot
	return slot
}

type LTable struct {
	Metatable LValue

	array   []LValue
	strdict map[string]*ltableSlot
	dict    map[[2]uintptr]*ltableSlot
	slots   ltableSlots
}

func (tb *LTable) String() string   { return fmt.Sprintf("table: %p", tb) }
func (tb *LTable) Type() LValueType { return LTTable }
func (tb *LTable) AsLValue() LValue {
	return LValue{dataptr: unsafe.Pointer(tb), data: maxUintptr + uintptr(LTTable)}
}

type LFunction struct {
	IsG       bool
	Env       *LTable
	Proto     *FunctionProto
	GFunction LGFunction
	Upvalues  []*Upvalue
}
type LGFunction func(*LState) int

func (fn *LFunction) String() string   { return fmt.Sprintf("function: %p", fn) }
func (fn *LFunction) Type() LValueType { return LTFunction }
func (fn *LFunction) AsLValue() LValue {
	return LValue{dataptr: unsafe.Pointer(fn), data: maxUintptr + uintptr(LTFunction)}
}

type Global struct {
	MainThread    *LState
	CurrentThread *LState
	Registry      *LTable
	Global        *LTable

	builtinMts map[int]LValue
	tempFiles  []*os.File
}

type LState struct {
	G       *Global
	Parent  *LState
	Env     *LTable
	Panic   func(*LState)
	Dead    bool
	Options Options

	stop         int32
	reg          *registry
	stack        *autoGrowingCallFrameStack
	alloc        *allocator
	currentFrame *callFrame
	wrapped      bool
	uvcache      *Upvalue
	hasErrorFunc bool
	mainLoop     func(*LState, *callFrame)
	ctx          context.Context
	ctxCancelFn  context.CancelFunc
}

func (ls *LState) String() string   { return fmt.Sprintf("thread: %p", ls) }
func (ls *LState) Type() LValueType { return LTThread }
func (ls *LState) AsLValue() LValue {
	return LValue{dataptr: unsafe.Pointer(ls), data: maxUintptr + uintptr(LTThread)}
}

type LUserData struct {
	Value     interface{}
	Env       *LTable
	Metatable LValue
}

func (ud *LUserData) String() string   { return fmt.Sprintf("userdata: %p", ud) }
func (ud *LUserData) Type() LValueType { return LTUserData }
func (ud *LUserData) AsLValue() LValue {
	return LValue{dataptr: unsafe.Pointer(ud), data: maxUintptr + uintptr(LTUserData)}
}

type LChannel chan LValue

func (ch LChannel) String() string   { return fmt.Sprintf("channel: %p", ch) }
func (ch LChannel) Type() LValueType { return LTChannel }
func (ch LChannel) AsLValue() LValue {
	var chptr = unsafe.Pointer(&ch)
	return LValue{dataptr: *(*unsafe.Pointer)(chptr), data: maxUintptr + uintptr(LTChannel)}
}
