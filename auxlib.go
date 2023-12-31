package lua

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

/* checkType {{{ */

func (ls *LState) CheckAny(n int) LValue {
	if n > ls.GetTop() {
		ls.ArgError(n, "value expected")
	}
	return ls.Get(n)
}

func (ls *LState) CheckInt(n int) int {
	v := ls.Get(n)
	if intv, ok := v.AsLNumber(); ok {
		return int(intv)
	}
	ls.TypeError(n, LTNumber)
	return 0
}

func (ls *LState) CheckInt64(n int) int64 {
	v := ls.Get(n)
	if intv, ok := v.AsLNumber(); ok {
		return int64(intv)
	}
	ls.TypeError(n, LTNumber)
	return 0
}

func (ls *LState) CheckNumber(n int) LNumber {
	v := ls.Get(n)
	if lv, ok := v.AsLNumber(); ok {
		return lv
	}
	if lv, ok := v.AsLString(); ok {
		if num, err := parseNumber(string(lv)); err == nil {
			return num
		}
	}
	ls.TypeError(n, LTNumber)
	return 0
}

func (ls *LState) CheckString(n int) string {
	v := ls.Get(n)
	if lv, ok := v.AsLString(); ok {
		return string(lv)
	} else if LVCanConvToString(v) {
		return LVAsString(v)
	}
	ls.TypeError(n, LTString)
	return ""
}

func (ls *LState) CheckBool(n int) bool {
	v := ls.Get(n)
	if lv, ok := v.AsLBool(); ok {
		return bool(lv)
	}
	ls.TypeError(n, LTBool)
	return false
}

func (ls *LState) CheckTable(n int) *LTable {
	v := ls.Get(n)
	if lv, ok := v.AsLTable(); ok {
		return lv
	}
	ls.TypeError(n, LTTable)
	return nil
}

func (ls *LState) CheckFunction(n int) *LFunction {
	v := ls.Get(n)
	if lv, ok := v.AsLFunction(); ok {
		return lv
	}
	ls.TypeError(n, LTFunction)
	return nil
}

func (ls *LState) CheckUserData(n int) *LUserData {
	v := ls.Get(n)
	if lv, ok := v.AsLUserData(); ok {
		return lv
	}
	ls.TypeError(n, LTUserData)
	return nil
}

func (ls *LState) CheckThread(n int) *LState {
	v := ls.Get(n)
	if lv, ok := v.AsLState(); ok {
		return lv
	}
	ls.TypeError(n, LTThread)
	return nil
}

func (ls *LState) CheckType(n int, typ LValueType) {
	v := ls.Get(n)
	if v.Type() != typ {
		ls.TypeError(n, typ)
	}
}

func (ls *LState) CheckTypes(n int, typs ...LValueType) {
	vt := ls.Get(n).Type()
	for _, typ := range typs {
		if vt == typ {
			return
		}
	}
	buf := []string{}
	for _, typ := range typs {
		buf = append(buf, typ.String())
	}
	ls.ArgError(n, strings.Join(buf, " or ")+" expected, got "+ls.Get(n).Type().String())
}

func (ls *LState) CheckOption(n int, options []string) int {
	str := ls.CheckString(n)
	for i, v := range options {
		if v == str {
			return i
		}
	}
	ls.ArgError(n, fmt.Sprintf("invalid option: %s (must be one of %s)", str, strings.Join(options, ",")))
	return 0
}

/* }}} */

/* optType {{{ */

func (ls *LState) OptInt(n int, d int) int {
	v := ls.Get(n)
	if v.EqualsLNil() {
		return d
	}
	if intv, ok := v.AsLNumber(); ok {
		return int(intv)
	}
	ls.TypeError(n, LTNumber)
	return 0
}

func (ls *LState) OptInt64(n int, d int64) int64 {
	v := ls.Get(n)
	if v.EqualsLNil() {
		return d
	}
	if intv, ok := v.AsLNumber(); ok {
		return int64(intv)
	}
	ls.TypeError(n, LTNumber)
	return 0
}

func (ls *LState) OptNumber(n int, d LNumber) LNumber {
	v := ls.Get(n)
	if v.EqualsLNil() {
		return d
	}
	if lv, ok := v.AsLNumber(); ok {
		return lv
	}
	ls.TypeError(n, LTNumber)
	return 0
}

func (ls *LState) OptString(n int, d string) string {
	v := ls.Get(n)
	if v.EqualsLNil() {
		return d
	}
	if lv, ok := v.AsLString(); ok {
		return string(lv)
	}
	ls.TypeError(n, LTString)
	return ""
}

func (ls *LState) OptBool(n int, d bool) bool {
	v := ls.Get(n)
	if v.EqualsLNil() {
		return d
	}
	if lv, ok := v.AsLBool(); ok {
		return bool(lv)
	}
	ls.TypeError(n, LTBool)
	return false
}

func (ls *LState) OptTable(n int, d *LTable) *LTable {
	v := ls.Get(n)
	if v.EqualsLNil() {
		return d
	}
	if lv, ok := v.AsLTable(); ok {
		return lv
	}
	ls.TypeError(n, LTTable)
	return nil
}

func (ls *LState) OptFunction(n int, d *LFunction) *LFunction {
	v := ls.Get(n)
	if v.EqualsLNil() {
		return d
	}
	if lv, ok := v.AsLFunction(); ok {
		return lv
	}
	ls.TypeError(n, LTFunction)
	return nil
}

func (ls *LState) OptUserData(n int, d *LUserData) *LUserData {
	v := ls.Get(n)
	if v.EqualsLNil() {
		return d
	}
	if lv, ok := v.AsLUserData(); ok {
		return lv
	}
	ls.TypeError(n, LTUserData)
	return nil
}

/* }}} */

/* error operations {{{ */

func (ls *LState) ArgError(n int, message string) {
	ls.RaiseError("bad argument #%v to %v (%v)", n, ls.rawFrameFuncName(ls.currentFrame), message)
}

func (ls *LState) TypeError(n int, typ LValueType) {
	ls.RaiseError("bad argument #%v to %v (%v expected, got %v)", n, ls.rawFrameFuncName(ls.currentFrame), typ.String(), ls.Get(n).Type().String())
}

/* }}} */

/* debug operations {{{ */

func (ls *LState) Where(level int) string {
	return ls.where(level, false)
}

/* }}} */

/* table operations {{{ */

func (ls *LState) FindTable(obj *LTable, n string, size int) LValue {
	names := strings.Split(n, ".")
	curobj := obj
	for _, name := range names {
		if curobj.Type() != LTTable {
			return LValue{}
		}
		nextobj := ls.RawGet(curobj, LString(name).AsLValue())
		if nextobj.EqualsLNil() {
			tb := ls.CreateTable(0, size)
			ls.RawSet(curobj, LString(name).AsLValue(), tb.AsLValue())
			curobj = tb
		} else if nextobj.Type() != LTTable {
			return LValue{}
		} else {
			curobj = nextobj.MustLTable()
		}
	}
	return curobj.AsLValue()
}

/* }}} */

/* register operations {{{ */

func (ls *LState) RegisterModule(name string, funcs map[string]LGFunction) LValue {
	tb := ls.FindTable(ls.Get(RegistryIndex).MustLTable(), "_LOADED", 1)
	mod := ls.GetField(tb, name)
	if mod.Type() != LTTable {
		newmod := ls.FindTable(ls.Get(GlobalsIndex).MustLTable(), name, len(funcs))
		if newmodtb, ok := newmod.AsLTable(); !ok {
			ls.RaiseError("name conflict for module(%v)", name)
		} else {
			for fname, fn := range funcs {
				newmodtb.RawSetString(fname, ls.NewFunction(fn).AsLValue())
			}
			ls.SetField(tb, name, newmodtb.AsLValue())
			return newmodtb.AsLValue()
		}
	}
	return mod
}

func (ls *LState) RegisterModuleWithSpecs(name string, specs map[string]LGFunctionSpec) LValue {
	tb := ls.FindTable(ls.Get(RegistryIndex).MustLTable(), "_LOADED", 1)
	mod := ls.GetField(tb, name)
	if mod.Type() != LTTable {
		newmod := ls.FindTable(ls.Get(GlobalsIndex).MustLTable(), name, len(specs))
		if newmodtb, ok := newmod.AsLTable(); !ok {
			ls.RaiseError("name conflict for module(%v)", name)
		} else {
			for fname, spec := range specs {
				newmodtb.RawSetString(fname, ls.NewFunctionSpec(spec).AsLValue())
			}
			ls.SetField(tb, name, newmodtb.AsLValue())
			return newmodtb.AsLValue()
		}
	}
	return mod
}

func (ls *LState) SetFuncs(tb *LTable, funcs map[string]LGFunction, upvalues ...LValue) *LTable {
	for fname, fn := range funcs {
		tb.RawSetString(fname, ls.NewClosure(fn, upvalues...).AsLValue())
	}
	return tb
}

/* }}} */

/* metatable operations {{{ */

func (ls *LState) NewTypeMetatable(typ string) *LTable {
	regtable := ls.Get(RegistryIndex)
	mt := ls.GetField(regtable, typ)
	if tb, ok := mt.AsLTable(); ok {
		return tb
	}
	mtnew := ls.NewTable()
	ls.SetField(regtable, typ, mtnew.AsLValue())
	return mtnew
}

func (ls *LState) GetMetaField(obj LValue, event string) LValue {
	return ls.metaOp1(obj, event)
}

func (ls *LState) GetTypeMetatable(typ string) LValue {
	return ls.GetField(ls.Get(RegistryIndex), typ)
}

func (ls *LState) CallMeta(obj LValue, event string) LValue {
	op := ls.metaOp1(obj, event)
	if op.Type() == LTFunction {
		ls.reg.Push(op)
		ls.reg.Push(obj)
		ls.Call(1, 1)
		return ls.reg.Pop()
	}
	return LValue{}
}

/* }}} */

/* load and function call operations {{{ */

func (ls *LState) LoadFile(path string) (*LFunction, error) {
	var file *os.File
	var err error
	if len(path) == 0 {
		file = os.Stdin
	} else {
		file, err = os.Open(path)
		if err != nil {
			return nil, newApiErrorE(ApiErrorFile, err)
		}
		defer file.Close()
	}

	reader := bufio.NewReader(file)
	// get the first character.
	c, err := reader.ReadByte()
	if err != nil && err != io.EOF {
		return nil, newApiErrorE(ApiErrorFile, err)
	}
	if c == byte('#') {
		// Unix exec. file?
		// skip first line
		_, err, _ = readBufioLine(reader)
		if err != nil {
			return nil, newApiErrorE(ApiErrorFile, err)
		}
	}

	if err != io.EOF {
		// if the file is not empty,
		// unread the first character of the file or newline character(readBufioLine's last byte).
		err = reader.UnreadByte()
		if err != nil {
			return nil, newApiErrorE(ApiErrorFile, err)
		}
	}

	return ls.Load(reader, path)
}

func (ls *LState) LoadString(source string) (*LFunction, error) {
	return ls.Load(strings.NewReader(source), "<string>")
}

func (ls *LState) DoFile(path string) error {
	if fn, err := ls.LoadFile(path); err != nil {
		return err
	} else {
		ls.Push(fn.AsLValue())
		return ls.PCall(0, MultRet, nil)
	}
}

func (ls *LState) DoString(source string) error {
	if fn, err := ls.LoadString(source); err != nil {
		return err
	} else {
		ls.Push(fn.AsLValue())
		return ls.PCall(0, MultRet, nil)
	}
}

/* }}} */

/* GopherLua original APIs {{{ */

// ToStringMeta returns string representation of given LValue.
// This method calls the `__tostring` meta method if defined.
func (ls *LState) ToStringMeta(lv LValue) LValue {
	if fn, ok := ls.metaOp1(lv, "__tostring").AsLFunction(); ok {
		ls.Push(fn.AsLValue())
		ls.Push(lv)
		ls.Call(1, 1)
		return ls.reg.Pop()
	} else {
		return LString(lv.String()).AsLValue()
	}
}

// Set a module loader to the package.preload table.
func (ls *LState) PreloadModule(name string, loader LGFunction) {
	preload := ls.GetField(ls.GetField(ls.Get(EnvironIndex), "package"), "preload")
	if _, ok := preload.AsLTable(); !ok {
		ls.RaiseError("package.preload must be a table")
	}
	ls.SetField(preload, name, ls.NewFunction(loader).AsLValue())
}

// Checks whether the given index is an LChannel and returns this channel.
func (ls *LState) CheckChannel(n int) chan LValue {
	v := ls.Get(n)
	if ch, ok := v.AsLChannel(); ok {
		return (chan LValue)(ch)
	}
	ls.TypeError(n, LTChannel)
	return nil
}

// If the given index is a LChannel, returns this channel. If this argument is absent or is nil, returns ch. Otherwise, raises an error.
func (ls *LState) OptChannel(n int, ch chan LValue) chan LValue {
	v := ls.Get(n)
	if v.EqualsLNil() {
		return ch
	}
	if ch, ok := v.AsLChannel(); ok {
		return (chan LValue)(ch)
	}
	ls.TypeError(n, LTChannel)
	return nil
}

/* }}} */

//

func NewTable() *LTable {
	return (*LState)(nil).NewTable()
}

func NewGFunction(fn LGFunction) *LFunction {
	return &LFunction{
		IsG: true, Env: AutoEnv(),
		GFunction: fn,
	}
}

var AutoEnv = sync.OnceValue(func() *LTable {
	T := new(LTable)
	mt := new(LTable)
	mt.RawSetString("__index", (&LFunction{
		IsG: true, Env: T,
		GFunction: autoEnv__index,
	}).AsLValue())
	T.Metatable = mt.AsLValue()
	return T
})

func autoEnv__index(L *LState) int {
	key := L.CheckString(2)
	L.Push(L.GetField(L.Env.AsLValue(), key))
	return 1
}
