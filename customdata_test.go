package lua

import (
	"strconv"
	"testing"
)

type MyInt int

var MyIntHelper *CustomDataHelper[MyInt]

func _MyInt__add(L *LState) int {
	a := MyIntHelper.Must(L.CheckAny(1))
	b := MyIntHelper.Must(L.CheckAny(2))
	c := *a + *b
	L.Push(MyIntHelper.AsLValue(&c))
	return 1
}

func __MyInt__tostring(L *LState) int {
	a := MyIntHelper.Must(L.CheckAny(1))
	L.Push(LString(strconv.Itoa(int(*a))).AsLValue())
	return 1
}

func init() {
	var L = (*LState)(nil)
	var mt = L.NewTable()
	mt.RawSetString("__add", (&LFunction{
		IsG:       true,
		GFunction: _MyInt__add,
	}).AsLValue())
	mt.RawSetString("__tostring", (&LFunction{
		IsG:       true,
		GFunction: __MyInt__tostring,
	}).AsLValue())
	MyIntHelper = RegisterCustomData[MyInt](mt)
}

func TestCustomData(t *testing.T) {
	L := NewState()
	defer L.Close()
	var a MyInt = 1
	var b MyInt = 2
	L.SetGlobal("a", MyIntHelper.AsLValue(&a))
	L.SetGlobal("b", MyIntHelper.AsLValue(&b))
	errorIfScriptFail(t, L, `
	c = a + b
	cstr = tostring(c)
	`)
	c := MyIntHelper.Must(L.GetGlobal("c"))
	cstr := L.GetGlobal("cstr").MustLString()
	errorIfFalse(t, *c == 3, "c must be 3")
	errorIfFalse(t, cstr == "3", "cstr must be 3")
}
