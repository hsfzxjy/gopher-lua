package lua

import (
	"context"
	"fmt"
	"math"
	"os"
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
	dataptr unsafe.Pointer
	data    uintptr
}

var (
	ltSentinels      = [4]int{}
	ltSentinelNumber = unsafe.Pointer(&ltSentinels[0])
	ltSentinelTrue   = unsafe.Pointer(&ltSentinels[1])
	ltSentinelFalse  = unsafe.Pointer(&ltSentinels[2])
	ltSentinelNil    = unsafe.Pointer(&ltSentinels[3])
)

const maxUintptr = ^uintptr(0)>>1 + 1

func AnyEqual(v1, v2 any) bool {
	if v1 == v2 {
		return true
	}
	v11, ok1 := v1.(LValue)
	v22, ok2 := v2.(LValue)
	if ok1 == ok2 {
		return false
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

func (lv LValue) Equals(other LValue) bool {
	if v1, ok1 := lv.AsLString(); !ok1 {
		return lv == other
	} else if v2, ok2 := other.AsLString(); !ok2 {
		return false
	} else {
		return v1 == v2
	}
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
	switch lv.Type() {
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
	default:
		return nil
	}
}

func (lv LValue) AsLValue() LValue { return lv }

func (lv LValue) Type() LValueType {
	if lv == (LValue{}) {
		return LTUnknown
	}
	if lv.dataptr == ltSentinelNumber {
		return LTNumber
	}
	if lv.data&maxUintptr == 0 || lv.data == maxUintptr+uintptr(LTString) {
		return LTString
	}
	typ := lv.data - maxUintptr
	if typ < uintptr(LTUnknown) {
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
	if lv.dataptr == ltSentinelNil {
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
	if lv.dataptr == ltSentinelTrue {
		return LTrue, true
	}
	if lv.dataptr == ltSentinelFalse {
		return LFalse, true
	}
	return LFalse, false
}

func (lv LValue) MustLNumber() LNumber {
	if v, ok := lv.AsLNumber(); ok {
		return v
	}
	panic("not number")
}

func (lv LValue) AsLNumber() (LNumber, bool) {
	if lv.dataptr == ltSentinelNumber {
		return LNumber(math.Float64frombits(uint64(lv.data))), true
	}
	return LNumber(0), false
}

func (lv LValue) MustLString() LString {
	if v, ok := lv.AsLString(); ok {
		return v
	}
	panic("not string")
}

func (lv LValue) AsLString() (LString, bool) {
	if lv.dataptr == ltSentinelNumber || lv == (LValue{}) {
		return "", false
	}
	if lv.data&maxUintptr == 0 {
		str := unsafe.String((*byte)(lv.dataptr), int(lv.data))
		return LString(str), true
	}
	if lv.data == maxUintptr+uintptr(LTString) {
		return "", true
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
	if lv.data != maxUintptr+uintptr(LTTable) {
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
	if lv.data != maxUintptr+uintptr(LTFunction) {
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
	if lv.data != maxUintptr+uintptr(LTUserData) {
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
	if lv.data != maxUintptr+uintptr(LTChannel) {
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
	if lv.data != maxUintptr+uintptr(LTThread) {
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
func LVIsFalse(v LValue) bool { return v.dataptr == ltSentinelNil || v.dataptr == ltSentinelFalse }

// LVIsFalse returns false if a given LValue is a nil or false otherwise true.
func LVAsBool(v LValue) bool { return v.dataptr != ltSentinelNil && v.dataptr != ltSentinelFalse }

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
func (nl *LNilType) AsLValue() LValue { return LNil }

var LNilValue = new(LNilType)
var LNil = LValue{ltSentinelNil, maxUintptr + uintptr(LTNil)}

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
		return LValue{ltSentinelTrue, maxUintptr + uintptr(LTBool)}
	} else {
		return LValue{ltSentinelFalse, maxUintptr + uintptr(LTBool)}
	}
}

var LTrue = LBool(true)
var LFalse = LBool(false)

type LString string

func (st LString) String() string   { return string(st) }
func (st LString) Type() LValueType { return LTString }
func (st LString) AsLValue() LValue {
	if st == "" {
		return LValue{nil, maxUintptr + uintptr(LTString)}
	}
	return LValue{unsafe.Pointer(unsafe.StringData(string(st))), uintptr(len(st))}
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
		return fmt.Sprint(int64(nm))
	}
	return fmt.Sprint(float64(nm))
}

func (nm LNumber) Type() LValueType { return LTNumber }
func (nm LNumber) AsLValue() LValue {
	return LValue{ltSentinelNumber, uintptr(math.Float64bits(float64(nm)))}
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

type LTable struct {
	Metatable LValue

	array   []LValue
	dict    map[LValue]LValue
	strdict map[string]LValue
	keys    []LValue
	k2i     map[LValue]int
}

func (tb *LTable) String() string   { return fmt.Sprintf("table: %p", tb) }
func (tb *LTable) Type() LValueType { return LTTable }
func (tb *LTable) AsLValue() LValue {
	return LValue{unsafe.Pointer(tb), maxUintptr + uintptr(LTTable)}
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
	return LValue{unsafe.Pointer(fn), maxUintptr + uintptr(LTFunction)}
}

type Global struct {
	MainThread    *LState
	CurrentThread *LState
	Registry      *LTable
	Global        *LTable

	builtinMts map[int]LValue
	tempFiles  []*os.File
	gccount    int32
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
	stack        callFrameStack
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
	return LValue{unsafe.Pointer(ls), maxUintptr + uintptr(LTThread)}
}

type LUserData struct {
	Value     interface{}
	Env       *LTable
	Metatable LValue
}

func (ud *LUserData) String() string   { return fmt.Sprintf("userdata: %p", ud) }
func (ud *LUserData) Type() LValueType { return LTUserData }
func (ud *LUserData) AsLValue() LValue {
	return LValue{unsafe.Pointer(ud), maxUintptr + uintptr(LTUserData)}
}

type LChannel chan LValue

func (ch LChannel) String() string   { return fmt.Sprintf("channel: %p", ch) }
func (ch LChannel) Type() LValueType { return LTChannel }
func (ch LChannel) AsLValue() LValue {
	var chptr = unsafe.Pointer(&ch)
	return LValue{*(*unsafe.Pointer)(chptr), maxUintptr + uintptr(LTChannel)}
}
