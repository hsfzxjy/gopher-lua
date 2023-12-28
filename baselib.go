package lua

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
)

/* basic functions {{{ */

func OpenBase(L *LState) int {
	global := L.Get(GlobalsIndex).MustLTable()
	L.SetGlobal("_G", global.AsLValue())
	L.SetGlobal("_VERSION", LString(LuaVersion).AsLValue())
	L.SetGlobal("_GOPHER_LUA_VERSION", LString(PackageName+" "+PackageVersion).AsLValue())
	basemod := L.RegisterModule("_G", baseFuncs)
	global.RawSetString("ipairs", L.NewClosure(baseIpairs, L.NewFunction(ipairsaux).AsLValue()).AsLValue())
	global.RawSetString("pairs", L.NewClosure(basePairs, L.NewFunction(pairsaux).AsLValue()).AsLValue())
	L.Push(basemod)
	return 1
}

var baseFuncs = map[string]LGFunction{
	"assert":         baseAssert,
	"collectgarbage": baseCollectGarbage,
	"dofile":         baseDoFile,
	"error":          baseError,
	"getfenv":        baseGetFEnv,
	"getmetatable":   baseGetMetatable,
	"load":           baseLoad,
	"loadfile":       baseLoadFile,
	"loadstring":     baseLoadString,
	"next":           baseNext,
	"pcall":          basePCall,
	"print":          basePrint,
	"rawequal":       baseRawEqual,
	"rawget":         baseRawGet,
	"rawset":         baseRawSet,
	"select":         baseSelect,
	"_printregs":     base_PrintRegs,
	"setfenv":        baseSetFEnv,
	"setmetatable":   baseSetMetatable,
	"tonumber":       baseToNumber,
	"tostring":       baseToString,
	"type":           baseType,
	"unpack":         baseUnpack,
	"xpcall":         baseXPCall,
	// loadlib
	"module":  loModule,
	"require": loRequire,
	// hidden features
	"newproxy": baseNewProxy,
}

func baseAssert(L *LState) int {
	if !L.ToBool(1) {
		L.RaiseError(L.OptString(2, "assertion failed!"))
		return 0
	}
	return L.GetTop()
}

func baseCollectGarbage(L *LState) int {
	runtime.GC()
	return 0
}

func baseDoFile(L *LState) int {
	src := L.ToString(1)
	top := L.GetTop()
	fn, err := L.LoadFile(src)
	if err != nil {
		L.Push(LString(err.Error()).AsLValue())
		L.Panic(L)
	}
	L.Push(fn.AsLValue())
	L.Call(0, MultRet)
	return L.GetTop() - top
}

func baseError(L *LState) int {
	obj := L.CheckAny(1)
	level := L.OptInt(2, 1)
	L.Error(obj, level)
	return 0
}

func baseGetFEnv(L *LState) int {
	var value LValue
	if L.GetTop() == 0 {
		value = LNumber(1).AsLValue()
	} else {
		value = L.Get(1)
	}

	if fn, ok := value.AsLFunction(); ok {
		if !fn.IsG {
			L.Push(fn.Env.AsLValue())
		} else {
			L.Push(L.G.Global.AsLValue())
		}
		return 1
	}

	if number, ok := value.AsLNumber(); ok {
		level := int(float64(number))
		if level <= 0 {
			L.Push(L.Env.AsLValue())
		} else {
			cf := L.currentFrame
			for i := 0; i < level && cf != nil; i++ {
				cf = cf.Parent
			}
			if cf == nil || cf.Fn.IsG {
				L.Push(L.G.Global.AsLValue())
			} else {
				L.Push(cf.Fn.Env.AsLValue())
			}
		}
		return 1
	}

	L.Push(L.G.Global.AsLValue())
	return 1
}

func baseGetMetatable(L *LState) int {
	L.Push(L.GetMetatable(L.CheckAny(1)))
	return 1
}

func ipairsaux(L *LState) int {
	tb := L.CheckTable(1)
	i := L.CheckInt(2)
	i++
	v := tb.RawGetInt(i)
	if v.EqualsLNil() {
		return 0
	} else {
		L.Pop(1)
		L.Push(LNumber(i).AsLValue())
		L.Push(LNumber(i).AsLValue())
		L.Push(v)
		return 2
	}
}

func baseIpairs(L *LState) int {
	tb := L.CheckTable(1)
	L.Push(L.Get(UpvalueIndex(1)))
	L.Push(tb.AsLValue())
	L.Push(LNumber(0).AsLValue())
	return 3
}

func loadaux(L *LState, reader io.Reader, chunkname string) int {
	if fn, err := L.Load(reader, chunkname); err != nil {
		L.Push(LNil)
		L.Push(LString(err.Error()).AsLValue())
		return 2
	} else {
		L.Push(fn.AsLValue())
		return 1
	}
}

func baseLoad(L *LState) int {
	fn := L.CheckFunction(1)
	chunkname := L.OptString(2, "?")
	top := L.GetTop()
	buf := []string{}
	for {
		L.SetTop(top)
		L.Push(fn.AsLValue())
		L.Call(0, 1)
		ret := L.reg.Pop()
		if ret.EqualsLNil() {
			break
		} else if LVCanConvToString(ret) {
			str := ret.String()
			if len(str) > 0 {
				buf = append(buf, string(str))
			} else {
				break
			}
		} else {
			L.Push(LNil)
			L.Push(LString("reader function must return a string").AsLValue())
			return 2
		}
	}
	return loadaux(L, strings.NewReader(strings.Join(buf, "")), chunkname)
}

func baseLoadFile(L *LState) int {
	var reader io.Reader
	var chunkname string
	var err error
	if L.GetTop() < 1 {
		reader = os.Stdin
		chunkname = "<stdin>"
	} else {
		chunkname = L.CheckString(1)
		reader, err = os.Open(chunkname)
		if err != nil {
			L.Push(LNil)
			L.Push(LString(fmt.Sprintf("can not open file: %v", chunkname)).AsLValue())
			return 2
		}
		defer reader.(*os.File).Close()
	}
	return loadaux(L, reader, chunkname)
}

func baseLoadString(L *LState) int {
	return loadaux(L, strings.NewReader(L.CheckString(1)), L.OptString(2, "<string>"))
}

func baseNext(L *LState) int {
	tb := L.CheckTable(1)
	index := LNil
	if L.GetTop() >= 2 {
		index = L.Get(2)
	}
	key, value := tb.Next(index)
	if key.EqualsLNil() {
		L.Push(LNil)
		return 1
	}
	L.Push(key)
	L.Push(value)
	return 2
}

func pairsaux(L *LState) int {
	tb := L.CheckTable(1)
	key, value := tb.Next(L.Get(2))
	if key.EqualsLNil() {
		return 0
	} else {
		L.Pop(1)
		L.Push(key)
		L.Push(key)
		L.Push(value)
		return 2
	}
}

func basePairs(L *LState) int {
	tb := L.CheckTable(1)
	L.Push(L.Get(UpvalueIndex(1)))
	L.Push(tb.AsLValue())
	L.Push(LNil)
	return 3
}

func basePCall(L *LState) int {
	L.CheckAny(1)
	v := L.Get(1)
	if v.Type() != LTFunction && L.GetMetaField(v, "__call").Type() != LTFunction {
		L.Push(LFalse.AsLValue())
		L.Push(LString("attempt to call a " + v.Type().String() + " value").AsLValue())
		return 2
	}
	nargs := L.GetTop() - 1
	if err := L.PCall(nargs, MultRet, nil); err != nil {
		L.Push(LFalse.AsLValue())
		if aerr, ok := err.(*ApiError); ok {
			L.Push(aerr.Object)
		} else {
			L.Push(LString(err.Error()).AsLValue())
		}
		return 2
	} else {
		L.Insert(LTrue.AsLValue(), 1)
		return L.GetTop()
	}
}

func basePrint(L *LState) int {
	top := L.GetTop()
	for i := 1; i <= top; i++ {
		fmt.Print(L.ToStringMeta(L.Get(i)).String())
		if i != top {
			fmt.Print("\t")
		}
	}
	fmt.Println("")
	return 0
}

func base_PrintRegs(L *LState) int {
	L.printReg()
	return 0
}

func baseRawEqual(L *LState) int {
	if L.CheckAny(1).Equals(L.CheckAny(2)) {
		L.Push(LTrue.AsLValue())
	} else {
		L.Push(LFalse.AsLValue())
	}
	return 1
}

func baseRawGet(L *LState) int {
	L.Push(L.RawGet(L.CheckTable(1), L.CheckAny(2)))
	return 1
}

func baseRawSet(L *LState) int {
	L.RawSet(L.CheckTable(1), L.CheckAny(2), L.CheckAny(3))
	return 0
}

func baseSelect(L *LState) int {
	L.CheckTypes(1, LTNumber, LTString)
	switch v := L.Get(1); v.Type() {
	case LTNumber:
		idx := int(v.MustLNumber())
		num := L.GetTop()
		if idx < 0 {
			idx = num + idx
		} else if idx > num {
			idx = num
		}
		if 1 > idx {
			L.ArgError(1, "index out of range")
		}
		return num - idx
	case LTString:
		if string(v.MustLString()) != "#" {
			L.ArgError(1, "invalid string '"+string(v.MustLString())+"'")
		}
		L.Push(LNumber(L.GetTop() - 1).AsLValue())
		return 1
	}
	return 0
}

func baseSetFEnv(L *LState) int {
	var value LValue
	if L.GetTop() == 0 {
		value = LNumber(1).AsLValue()
	} else {
		value = L.Get(1)
	}
	env := L.CheckTable(2)

	if fn, ok := value.AsLFunction(); ok {
		if fn.IsG {
			L.RaiseError("cannot change the environment of given object")
		} else {
			fn.Env = env
			L.Push(fn.AsLValue())
			return 1
		}
	}

	if number, ok := value.AsLNumber(); ok {
		level := int(float64(number))
		if level <= 0 {
			L.Env = env
			return 0
		}

		cf := L.currentFrame
		for i := 0; i < level && cf != nil; i++ {
			cf = cf.Parent
		}
		if cf == nil || cf.Fn.IsG {
			L.RaiseError("cannot change the environment of given object")
		} else {
			cf.Fn.Env = env
			L.Push(cf.Fn.AsLValue())
			return 1
		}
	}

	L.RaiseError("cannot change the environment of given object")
	return 0
}

func baseSetMetatable(L *LState) int {
	L.CheckTypes(2, LTNil, LTTable)
	obj := L.Get(1)
	if obj.EqualsLNil() {
		L.RaiseError("cannot set metatable to a nil object.")
	}
	mt := L.Get(2)
	if m := L.metatable(obj, true); !m.EqualsLNil() {
		if tb, ok := m.AsLTable(); ok && !tb.RawGetString("__metatable").EqualsLNil() {
			L.RaiseError("cannot change a protected metatable")
		}
	}
	L.SetMetatable(obj, mt)
	L.SetTop(1)
	return 1
}

func baseToNumber(L *LState) int {
	base := L.OptInt(2, 10)
	noBase := L.Get(2).EqualsLNil()

	switch lv := L.CheckAny(1); lv.Type() {
	case LTNumber:
		L.Push(lv)
	case LTString:
		str := strings.Trim(string(lv.MustLString()), " \n\t")
		if strings.ContainsRune(str, '.') {
			if v, err := strconv.ParseFloat(str, LNumberBit); err != nil {
				L.Push(LNil)
			} else {
				L.Push(LNumber(v).AsLValue())
			}
		} else {
			if noBase && strings.HasPrefix(strings.ToLower(str), "0x") {
				base, str = 16, str[2:] // Hex number
			}
			if v, err := strconv.ParseInt(str, base, LNumberBit); err != nil {
				L.Push(LNil)
			} else {
				L.Push(LNumber(v).AsLValue())
			}
		}
	default:
		L.Push(LNil)
	}
	return 1
}

func baseToString(L *LState) int {
	v1 := L.CheckAny(1)
	L.Push(L.ToStringMeta(v1))
	return 1
}

func baseType(L *LState) int {
	L.Push(LString(L.CheckAny(1).Type().String()).AsLValue())
	return 1
}

func baseUnpack(L *LState) int {
	tb := L.CheckTable(1)
	start := L.OptInt(2, 1)
	end := L.OptInt(3, tb.Len())
	for i := start; i <= end; i++ {
		L.Push(tb.RawGetInt(i))
	}
	ret := end - start + 1
	if ret < 0 {
		return 0
	}
	return ret
}

func baseXPCall(L *LState) int {
	fn := L.CheckFunction(1)
	errfunc := L.CheckFunction(2)

	top := L.GetTop()
	L.Push(fn.AsLValue())
	if err := L.PCall(0, MultRet, errfunc); err != nil {
		L.Push(LFalse.AsLValue())
		if aerr, ok := err.(*ApiError); ok {
			L.Push(aerr.Object)
		} else {
			L.Push(LString(err.Error()).AsLValue())
		}
		return 2
	} else {
		L.Insert(LTrue.AsLValue(), top+1)
		return L.GetTop() - top
	}
}

/* }}} */

/* load lib {{{ */

func loModule(L *LState) int {
	name := L.CheckString(1)
	loaded := L.GetField(L.Get(RegistryIndex), "_LOADED")
	tb := L.GetField(loaded, name)
	if _, ok := tb.AsLTable(); !ok {
		tb = L.FindTable(L.Get(GlobalsIndex).MustLTable(), name, 1)
		if tb.EqualsLNil() {
			L.RaiseError("name conflict for module: %v", name)
		}
		L.SetField(loaded, name, tb)
	}
	if L.GetField(tb, "_NAME").EqualsLNil() {
		L.SetField(tb, "_M", tb)
		L.SetField(tb, "_NAME", LString(name).AsLValue())
		names := strings.Split(name, ".")
		pname := ""
		if len(names) > 1 {
			pname = strings.Join(names[:len(names)-1], ".") + "."
		}
		L.SetField(tb, "_PACKAGE", LString(pname).AsLValue())
	}

	caller := L.currentFrame.Parent
	if caller == nil {
		L.RaiseError("no calling stack.")
	} else if caller.Fn.IsG {
		L.RaiseError("module() can not be called from GFunctions.")
	}
	L.SetFEnv(caller.Fn.AsLValue(), tb)

	top := L.GetTop()
	for i := 2; i <= top; i++ {
		L.Push(L.Get(i))
		L.Push(tb)
		L.Call(1, 0)
	}
	L.Push(tb)
	return 1
}

var loopdetection = &LUserData{}

func loRequire(L *LState) int {
	name := L.CheckString(1)
	loaded := L.GetField(L.Get(RegistryIndex), "_LOADED")
	lv := L.GetField(loaded, name)
	if LVAsBool(lv) {
		if lv.Equals(loopdetection.AsLValue()) {
			L.RaiseError("loop or previous error loading module: %s", name)
		}
		L.Push(lv)
		return 1
	}
	loaders, ok := L.GetField(L.Get(RegistryIndex), "_LOADERS").AsLTable()
	if !ok {
		L.RaiseError("package.loaders must be a table")
	}
	messages := []string{}
	var modasfunc LValue
	for i := 1; ; i++ {
		loader := L.RawGetInt(loaders, i)
		if loader.EqualsLNil() {
			L.RaiseError("module %s not found:\n\t%s, ", name, strings.Join(messages, "\n\t"))
		}
		L.Push(loader)
		L.Push(LString(name).AsLValue())
		L.Call(1, 1)
		ret := L.reg.Pop()
		switch ret.Type() {
		case LTFunction:
			modasfunc = ret
			goto loopbreak
		case LTString:
			messages = append(messages, string(ret.MustLString()))
		}
	}
loopbreak:
	L.SetField(loaded, name, loopdetection.AsLValue())
	L.Push(modasfunc)
	L.Push(LString(name).AsLValue())
	L.Call(1, 1)
	ret := L.reg.Pop()
	modv := L.GetField(loaded, name)
	if !ret.EqualsLNil() && modv.Equals(loopdetection.AsLValue()) {
		L.SetField(loaded, name, ret)
		L.Push(ret)
	} else if modv.Equals(loopdetection.AsLValue()) {
		L.SetField(loaded, name, LTrue.AsLValue())
		L.Push(LTrue.AsLValue())
	} else {
		L.Push(modv)
	}
	return 1
}

/* }}} */

/* hidden features {{{ */

func baseNewProxy(L *LState) int {
	ud := L.NewUserData()
	L.SetTop(1)
	if L.Get(1).Equals(LTrue.AsLValue()) {
		L.SetMetatable(ud.AsLValue(), L.NewTable().AsLValue())
	} else if d, ok := L.Get(1).AsLUserData(); ok {
		L.SetMetatable(ud.AsLValue(), L.GetMetatable(d.AsLValue()))
	}
	L.Push(ud.AsLValue())
	return 1
}

/* }}} */

//
