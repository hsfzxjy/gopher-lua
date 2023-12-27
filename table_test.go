package lua

import (
	"strconv"
	"testing"
)

func TestTableNewLTable(t *testing.T) {
	tbl := newLTable(-1, -2)
	errorIfNotEqual(t, 0, cap(tbl.array))

	tbl = newLTable(10, 9)
	errorIfNotEqual(t, 10, cap(tbl.array))
}

func TestTableLen(t *testing.T) {
	tbl := newLTable(0, 0)
	tbl.RawSetInt(10, LNil)
	tbl.RawSetInt(9, LNumber(10).AsLValue())
	tbl.RawSetInt(8, LNil)
	tbl.RawSetInt(7, LNumber(10).AsLValue())
	errorIfNotEqual(t, 9, tbl.Len())

	tbl = newLTable(0, 0)
	tbl.Append(LTrue.AsLValue())
	tbl.Append(LTrue.AsLValue())
	tbl.Append(LTrue.AsLValue())
	errorIfNotEqual(t, 3, tbl.Len())
}

func TestTableLenType(t *testing.T) {
	L := NewState(Options{})
	err := L.DoString(`
        mt = {
            __index = mt,
            __len = function (self)
                return {hello = "world"}
            end
        }

        v = {}
        v.__index = v

        setmetatable(v, mt)

        assert(#v ~= 0, "#v should return a table reference in this case")

        print(#v)
    `)
	if err != nil {
		t.Error(err)
	}
}

func TestTableAppend(t *testing.T) {
	tbl := newLTable(0, 0)
	tbl.RawSetInt(1, LNumber(1).AsLValue())
	tbl.RawSetInt(2, LNumber(2).AsLValue())
	tbl.RawSetInt(3, LNumber(3).AsLValue())
	errorIfNotEqual(t, 3, tbl.Len())

	tbl.RawSetInt(1, LNil)
	tbl.RawSetInt(2, LNil)
	errorIfNotEqual(t, 3, tbl.Len())

	tbl.Append(LNumber(4).AsLValue())
	errorIfNotEqual(t, 4, tbl.Len())

	tbl.RawSetInt(3, LNil)
	tbl.RawSetInt(4, LNil)
	errorIfNotEqual(t, 0, tbl.Len())

	tbl.Append(LNumber(5).AsLValue())
	errorIfNotEqual(t, 1, tbl.Len())
}

func TestTableInsert(t *testing.T) {
	tbl := newLTable(0, 0)
	tbl.Append(LTrue.AsLValue())
	tbl.Append(LTrue.AsLValue())
	tbl.Append(LTrue.AsLValue())

	tbl.Insert(5, LFalse.AsLValue())
	errorIfNotEqual(t, LFalse, tbl.RawGetInt(5))
	errorIfNotEqual(t, 5, tbl.Len())

	tbl.Insert(-10, LFalse.AsLValue())
	errorIfNotEqual(t, LFalse, tbl.RawGet(LNumber(-10).AsLValue()))
	errorIfNotEqual(t, 5, tbl.Len())

	tbl = newLTable(0, 0)
	tbl.Append(LNumber(1).AsLValue())
	tbl.Append(LNumber(2).AsLValue())
	tbl.Append(LNumber(3).AsLValue())
	tbl.Insert(1, LNumber(10).AsLValue())
	errorIfNotEqual(t, LNumber(10), tbl.RawGetInt(1))
	errorIfNotEqual(t, LNumber(1), tbl.RawGetInt(2))
	errorIfNotEqual(t, LNumber(2), tbl.RawGetInt(3))
	errorIfNotEqual(t, LNumber(3), tbl.RawGetInt(4))
	errorIfNotEqual(t, 4, tbl.Len())

	tbl = newLTable(0, 0)
	tbl.Insert(5, LNumber(10).AsLValue())
	errorIfNotEqual(t, LNumber(10), tbl.RawGetInt(5))

}

func TestTableMaxN(t *testing.T) {
	tbl := newLTable(0, 0)
	tbl.Append(LTrue.AsLValue())
	tbl.Append(LTrue.AsLValue())
	tbl.Append(LTrue.AsLValue())
	errorIfNotEqual(t, 3, tbl.MaxN())

	tbl = newLTable(0, 0)
	errorIfNotEqual(t, 0, tbl.MaxN())

	tbl = newLTable(10, 0)
	errorIfNotEqual(t, 0, tbl.MaxN())
}

func TestTableRemove(t *testing.T) {
	tbl := newLTable(0, 0)
	errorIfNotEqual(t, LNil, tbl.Remove(10))
	tbl.Append(LTrue.AsLValue())
	errorIfNotEqual(t, LNil, tbl.Remove(10))

	tbl.Append(LFalse.AsLValue())
	tbl.Append(LTrue.AsLValue())
	errorIfNotEqual(t, LFalse, tbl.Remove(2))
	errorIfNotEqual(t, 2, tbl.MaxN())
	tbl.Append(LFalse.AsLValue())
	errorIfNotEqual(t, LFalse, tbl.Remove(-1))
	errorIfNotEqual(t, 2, tbl.MaxN())

}

func TestTableRawSetInt(t *testing.T) {
	tbl := newLTable(0, 0)
	tbl.RawSetInt(MaxArrayIndex+1, LTrue.AsLValue())
	errorIfNotEqual(t, 0, tbl.MaxN())
	errorIfNotEqual(t, LTrue, tbl.RawGet(LNumber(MaxArrayIndex+1).AsLValue()))

	tbl.RawSetInt(1, LTrue.AsLValue())
	tbl.RawSetInt(3, LTrue.AsLValue())
	errorIfNotEqual(t, 3, tbl.MaxN())
	errorIfNotEqual(t, LTrue, tbl.RawGetInt(1))
	errorIfNotEqual(t, LNil, tbl.RawGetInt(2))
	errorIfNotEqual(t, LTrue, tbl.RawGetInt(3))
	tbl.RawSetInt(2, LTrue.AsLValue())
	errorIfNotEqual(t, LTrue, tbl.RawGetInt(1))
	errorIfNotEqual(t, LTrue, tbl.RawGetInt(2))
	errorIfNotEqual(t, LTrue, tbl.RawGetInt(3))
}

func TestTableRawSetH(t *testing.T) {
	tbl := newLTable(0, 0)
	tbl.RawSetH(LString("key").AsLValue(), LTrue.AsLValue())
	tbl.RawSetH(LString("key").AsLValue(), LNil)
	_, found := tbl.dict[LString("key").AsLValue().asComparable()]
	errorIfNotEqual(t, false, found)

	tbl.RawSetH(LTrue.AsLValue(), LTrue.AsLValue())
	tbl.RawSetH(LTrue.AsLValue(), LNil)
	_, foundb := tbl.dict[LTrue.AsLValue().asComparable()]
	errorIfNotEqual(t, false, foundb)
}

func TestTableRawGetH(t *testing.T) {
	tbl := newLTable(0, 0)
	errorIfNotEqual(t, LNil, tbl.RawGetH(LNumber(1).AsLValue()))
	errorIfNotEqual(t, LNil, tbl.RawGetH(LString("key0").AsLValue()))
	tbl.RawSetH(LString("key0").AsLValue(), LTrue.AsLValue())
	tbl.RawSetH(LString("key1").AsLValue(), LFalse.AsLValue())
	tbl.RawSetH(LNumber(1).AsLValue(), LTrue.AsLValue())
	errorIfNotEqual(t, LTrue, tbl.RawGetH(LString("key0").AsLValue()))
	errorIfNotEqual(t, LTrue, tbl.RawGetH(LNumber(1).AsLValue()))
	errorIfNotEqual(t, LNil, tbl.RawGetH(LString("notexist").AsLValue()))
	errorIfNotEqual(t, LNil, tbl.RawGetH(LTrue.AsLValue()))
}

func TestTableForEach(t *testing.T) {
	tbl := newLTable(0, 0)
	tbl.Append(LNumber(1).AsLValue())
	tbl.Append(LNumber(2).AsLValue())
	tbl.Append(LNumber(3).AsLValue())
	tbl.Append(LNil)
	tbl.Append(LNumber(5).AsLValue())

	tbl.RawSetH(LString("a").AsLValue(), LString("a").AsLValue())
	tbl.RawSetH(LString("b").AsLValue(), LString("b").AsLValue())
	tbl.RawSetH(LString("c").AsLValue(), LString("c").AsLValue())

	tbl.RawSetH(LTrue.AsLValue(), LString("true").AsLValue())
	tbl.RawSetH(LFalse.AsLValue(), LString("false").AsLValue())

	var numberCounter = map[LNumber]int{
		1: 0, 2: 0, 3: 0, 4: 0,
	}
	var strCounter = map[LString]int{
		"a": 0, "b": 0, "c": 0,
	}
	var boolCounter = map[LBool]int{
		LTrue:  0,
		LFalse: 0,
	}

	tbl.ForEach(func(key, value LValue) {
		switch k := key; k.Type() {
		case LTBool:
			switch bool(k.MustLBool()) {
			case true:
				errorIfNotEqual(t, LString("true"), value)
			case false:
				errorIfNotEqual(t, LString("false"), value)
			default:
				t.Fail()
			}
			boolCounter[k.MustLBool()]++
		case LTNumber:
			switch int(k.MustLNumber()) {
			case 1:
				errorIfNotEqual(t, LNumber(1), value)
			case 2:
				errorIfNotEqual(t, LNumber(2), value)
			case 3:
				errorIfNotEqual(t, LNumber(3), value)
			case 4:
				errorIfNotEqual(t, LNumber(5), value)
			default:
				t.Fail()
			}
			numberCounter[k.MustLNumber()]++
		case LTString:
			switch string(k.MustLString()) {
			case "a":
				errorIfNotEqual(t, LString("a"), value)
			case "b":
				errorIfNotEqual(t, LString("b"), value)
			case "c":
				errorIfNotEqual(t, LString("c"), value)
			default:
				t.Fail()
			}
			strCounter[k.MustLString()]++
		}
	})

	for _, v := range numberCounter {
		errorIfNotEqual(t, 1, v)
	}
	for _, v := range strCounter {
		errorIfNotEqual(t, 1, v)
	}
	for _, v := range boolCounter {
		errorIfNotEqual(t, 1, v)
	}
}

func TestTableGrow(t *testing.T) {
	tbl := newLTable(0, 0)

	for i := 1; i <= blockSize; i++ {
		tbl.RawSetString(strconv.Itoa(i), LNumber(i).AsLValue())
	}
	errorIfNotEqual(t, 1, len(tbl.slots.blocks))
	tbl.RawSetString(strconv.Itoa(1), LNil)
	errorIfNotEqual(t, 1, len(tbl.slots.blocks))
}
