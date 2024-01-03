package lua

import (
	"fmt"
	"math"
	"strings"
	"unsafe"
)

var _ = unsafe.Pointer(nil)

const (
	retOK = iota
	retFail
	retFnChanged
	retCFChanged
)

func mainLoop(L *LState, baseframe *callFrame) {
	var inst uint32

	if L.stack.IsEmpty() {
		return
	}

	L.currentFrame = L.stack.Last()
	if L.currentFrame.Fn.IsG {
		callGFunction(L, false)
		return
	}

	var (
		cf   = L.currentFrame
		code = cf.Fn.Proto.Code
		ret  int
		jt   = &jumpTable
	)

	for {
		inst = code[cf.Pc]
		cf.Pc++
		ret = jt[int(inst>>26)](L, inst, baseframe)
		if ret == retOK {
			continue
		}
		switch ret {
		case retFail:
			return
		case retFnChanged:
			code = cf.Fn.Proto.Code
		case retCFChanged:
			cf = L.currentFrame
			code = cf.Fn.Proto.Code
		}
	}
}

func mainLoopWithContext(L *LState, baseframe *callFrame) {
	var inst uint32

	if L.stack.IsEmpty() {
		return
	}

	L.currentFrame = L.stack.Last()
	if L.currentFrame.Fn.IsG {
		callGFunction(L, false)
		return
	}

	var (
		cf   = L.currentFrame
		code = cf.Fn.Proto.Code
		ret  int
		jt   = &jumpTable
	)

	for {
		inst = code[cf.Pc]
		cf.Pc++
		select {
		case <-L.ctx.Done():
			L.RaiseError(L.ctx.Err().Error())
			return
		default:
			ret = jt[int(inst>>26)](L, inst, baseframe)
			if ret == retOK {
				continue
			}
			switch ret {
			case retFail:
				return
			case retFnChanged:
				code = cf.Fn.Proto.Code
			case retCFChanged:
				cf = L.currentFrame
				code = cf.Fn.Proto.Code
			}
		}
	}
}

// regv is the first target register to copy the return values to.
// It can be reg.top, indicating that the copied values are going into new registers, or it can be below reg.top
// Indicating that the values should be within the existing registers.
// b is the available number of return values + 1.
// n is the desired number of return values.
// If n more than the available return values then the extra values are set to nil.
// When this function returns the top of the registry will be set to regv+n.
//
//lint:ignore U1000 For inlining
func copyReturnValues(L *LState, regv, start, n, b int) { // +inline-start
	if b == 1 {
		// +inline-call L.reg.FillNil  regv n
	} else {
		// +inline-call L.reg.CopyRange regv start -1 n
		if b > 1 && n > (b-1) {
			// +inline-call L.reg.FillNil  regv+b-1 n-(b-1)
		}
	}
} // +inline-end

func switchToParentThread(L *LState, nargs int, haserror bool, kill bool) {
	parent := L.Parent
	if parent == nil {
		L.RaiseError("can not yield from outside of a coroutine")
	}
	L.G.CurrentThread = parent
	L.Parent = nil
	if !L.wrapped {
		if haserror {
			parent.Push(LFalse.AsLValue())
		} else {
			parent.Push(LTrue.AsLValue())
		}
	}
	L.XMoveTo(parent, nargs)
	L.stack.Pop()
	offset := L.currentFrame.LocalBase - L.currentFrame.ReturnBase
	L.currentFrame = L.stack.Last()
	L.reg.SetTop(L.reg.Top() - offset) // remove 'yield' function(including tailcalled functions)
	if kill {
		L.kill()
	}
}

func callGFunction(L *LState, tailcall bool) bool {
	frame := L.currentFrame
	gfnret := frame.Fn.GFunction(L)
	if tailcall {
		L.currentFrame = L.RemoveCallerFrame()
	}

	if gfnret < 0 {
		switchToParentThread(L, L.GetTop(), false, false)
		return true
	}

	wantret := frame.NRet
	if wantret == MultRet {
		wantret = gfnret
	}

	if tailcall && L.Parent != nil && L.stack.Sp() == 1 {
		switchToParentThread(L, wantret, false, true)
		return true
	}

	// +inline-call L.reg.CopyRange frame.ReturnBase L.reg.Top()-gfnret -1 wantret
	L.currentFrame = L.stack.Pop().Parent
	return false
}

func callFastGFunction(L *LState, fn *LFunction, RA, nret int, tailcall bool) bool {
	L.fastCallLBase = RA + 1
	gfnret := fn.GFunction(L)

	if nret == MultRet {
		nret = gfnret
	}
	// +inline-call L.reg.CopyRange RA L.reg.Top()-gfnret -1 nret
	L.fastCallLBase = 0
	return false
}

func threadRun(L *LState) {
	if L.stack.IsEmpty() {
		return
	}

	defer func() {
		if rcv := recover(); rcv != nil {
			var lv LValue
			if v, ok := rcv.(*ApiError); ok {
				lv = v.Object
			} else {
				lv = LString(fmt.Sprint(rcv)).AsLValue()
			}
			if parent := L.Parent; parent != nil {
				if L.wrapped {
					L.Push(lv)
					parent.Panic(L)
				} else {
					L.SetTop(0)
					L.Push(lv)
					switchToParentThread(L, 1, true, true)
				}
			} else {
				panic(rcv)
			}
		}
	}()
	L.mainLoop(L, nil)
}

type instFunc func(*LState, uint32, *callFrame) int

var jumpTable [opCodeMax + 1]instFunc

func init() {
	jumpTable = [opCodeMax + 1]instFunc{
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_MOVE
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff) //GETB
			v := reg.Get(lbase + B)
			// +inline-call reg.Set RA v
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_MOVEN
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			B := int(inst & 0x1ff)    //GETB
			C := int(inst>>9) & 0x1ff //GETC
			v := reg.Get(lbase + B)
			// +inline-call reg.Set lbase+A v
			code := cf.Fn.Proto.Code
			pc := cf.Pc
			for i := 0; i < C; i++ {
				inst = code[pc]
				pc++
				A = int(inst>>18) & 0xff //GETA
				B = int(inst & 0x1ff)    //GETB
				v := reg.Get(lbase + B)
				// +inline-call reg.Set lbase+A v
			}
			cf.Pc = pc
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_LOADK
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			Bx := int(inst & 0x3ffff) //GETBX
			v := cf.Fn.Proto.Constants[Bx]
			// +inline-call reg.Set RA v
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_LOADBOOL
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff)    //GETB
			C := int(inst>>9) & 0x1ff //GETC
			if B != 0 {
				// +inline-call reg.Set RA LTrue
			} else {
				// +inline-call reg.Set RA LFalse
			}
			if C != 0 {
				cf.Pc++
			}
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_LOADNIL
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff) //GETB
			for i := RA; i <= lbase+B; i++ {
				// +inline-call reg.Set i LNil
			}
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_GETUPVAL
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff) //GETB
			v := cf.Fn.Upvalues[B].Value()
			// +inline-call reg.Set RA v
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_GETGLOBAL
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			Bx := int(inst & 0x3ffff) //GETBX
			//reg.Set(RA, L.getField(cf.Fn.Env, cf.Fn.Proto.Constants[Bx]))
			v := L.getFieldString(cf.Fn.Env.AsLValue(), cf.Fn.Proto.stringConstants[Bx])
			// +inline-call reg.Set RA v
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_GETTABLE
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff)    //GETB
			C := int(inst>>9) & 0x1ff //GETC
			v := L.getField(reg.Get(lbase+B), L.rkValue(C))
			// +inline-call reg.Set RA v
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_GETTABLEKS
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff)    //GETB
			C := int(inst>>9) & 0x1ff //GETC
			v := L.getFieldString(reg.Get(lbase+B), L.rkString(C))
			// +inline-call reg.Set RA v
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_SETGLOBAL
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			Bx := int(inst & 0x3ffff) //GETBX
			//L.setField(cf.Fn.Env, cf.Fn.Proto.Constants[Bx], reg.Get(RA))
			L.setFieldString(cf.Fn.Env.AsLValue(), cf.Fn.Proto.stringConstants[Bx], reg.Get(RA))
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_SETUPVAL
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff) //GETB
			cf.Fn.Upvalues[B].SetValue(reg.Get(RA))
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_SETTABLE
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff)    //GETB
			C := int(inst>>9) & 0x1ff //GETC
			AObj := reg.Get(RA)
			BObj := L.rkValue(B)
			CObj := L.rkValue(C)
			if tb, ok := AObj.AsLTable(); ok && tb.Metatable.EqualsLNil() {
				if str, ok := BObj.AsLString(); ok {
					tb.RawSetString(string(str), CObj)
				} else {
					L.RawSet(tb, BObj, CObj)
				}
			} else {
				L.setField(AObj, BObj, CObj)
			}
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_SETTABLEKS
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff)    //GETB
			C := int(inst>>9) & 0x1ff //GETC
			L.setFieldString(reg.Get(RA), L.rkString(B), L.rkValue(C))
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_NEWTABLE
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff)    //GETB
			C := int(inst>>9) & 0x1ff //GETC
			v := newLTable(B, C)
			// +inline-call reg.Set RA v
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_SELF
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff)    //GETB
			C := int(inst>>9) & 0x1ff //GETC
			selfobj := reg.Get(lbase + B)
			v := L.getFieldString(selfobj, L.rkString(C))
			// +inline-call reg.Set RA v
			// +inline-call reg.Set RA+1 selfobj
			return 0
		},
		opArith, // OP_ADD
		opArith, // OP_SUB
		opArith, // OP_MUL
		opArith, // OP_DIV
		opArith, // OP_MOD
		opArith, // OP_POW
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_UNM
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff) //GETB
			unaryv := L.rkValue(B)
			if nm, ok := unaryv.AsLNumber(); ok {
				// +inline-call reg.Set RA -nm
			} else {
				op := L.metaOp1(unaryv, "__unm")
				if op.Type() == LTFunction {
					reg.Push(op)
					reg.Push(unaryv)
					L.Call(1, 1)
					// +inline-call reg.Set RA reg.Pop()
				} else if str, ok1 := unaryv.AsLString(); ok1 {
					if num, err := parseNumber(string(str)); err == nil {
						// +inline-call reg.Set RA -num
					} else {
						L.RaiseError("__unm undefined")
					}
				} else {
					L.RaiseError("__unm undefined")
				}
			}
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_NOT
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff) //GETB
			if LVIsFalse(reg.Get(lbase + B)) {
				// +inline-call reg.Set RA LTrue
			} else {
				// +inline-call reg.Set RA LFalse
			}
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_LEN
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff) //GETB
			switch lv := L.rkValue(B); lv.Type() {
			case LTString:
				lv := lv.MustLString()
				// +inline-call reg.SetNumber RA LNumber(len(lv))
			default:
				op := L.metaOp1(lv, "__len")
				if op.Type() == LTFunction {
					reg.Push(op)
					reg.Push(lv)
					L.Call(1, 1)
					ret := reg.Pop()
					if ret.Type() == LTNumber {
						v, _ := ret.AsLNumber()
						// +inline-call reg.SetNumber RA v
					} else {
						// +inline-call reg.Set RA ret
					}
				} else if lv.Type() == LTTable {
					// +inline-call reg.SetNumber RA LNumber(lv.MustLTable().Len())
				} else {
					L.RaiseError("__len undefined")
				}
			}
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_CONCAT
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff)    //GETB
			C := int(inst>>9) & 0x1ff //GETC
			RC := lbase + C
			RB := lbase + B
			v := stringConcat(L, RC-RB+1, RC)
			// +inline-call reg.Set RA v
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_JMP
			cf := L.currentFrame
			Sbx := int(inst&0x3ffff) - opMaxArgSbx //GETSBX
			cf.Pc += Sbx
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_EQ
			cf := L.currentFrame
			A := int(inst>>18) & 0xff //GETA
			B := int(inst & 0x1ff)    //GETB
			C := int(inst>>9) & 0x1ff //GETC
			ret := equals(L, L.rkValue(B), L.rkValue(C), false)
			v := 1
			if ret {
				v = 0
			}
			if v == A {
				cf.Pc++
			}
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_LT
			cf := L.currentFrame
			A := int(inst>>18) & 0xff //GETA
			B := int(inst & 0x1ff)    //GETB
			C := int(inst>>9) & 0x1ff //GETC
			ret := lessThan(L, L.rkValue(B), L.rkValue(C))
			v := 1
			if ret {
				v = 0
			}
			if v == A {
				cf.Pc++
			}
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_LE
			cf := L.currentFrame
			A := int(inst>>18) & 0xff //GETA
			B := int(inst & 0x1ff)    //GETB
			C := int(inst>>9) & 0x1ff //GETC
			lhs := L.rkValue(B)
			rhs := L.rkValue(C)
			ret := false

			if v1, ok1 := lhs.AsLNumber(); ok1 {
				if v2, ok2 := rhs.AsLNumber(); ok2 {
					ret = v1 <= v2
				} else {
					L.RaiseError("attempt to compare %v with %v", lhs.Type().String(), rhs.Type().String())
				}
			} else {
				if lhs.Type() != rhs.Type() {
					L.RaiseError("attempt to compare %v with %v", lhs.Type().String(), rhs.Type().String())
				}
				switch lhs.Type() {
				case LTString:
					ret = strCmp(string(lhs.MustLString()), string(rhs.MustLString())) <= 0
				default:
					switch objectRational(L, lhs, rhs, "__le") {
					case 1:
						ret = true
					case 0:
						ret = false
					default:
						ret = !objectRationalWithError(L, rhs, lhs, "__lt")
					}
				}
			}

			v := 1
			if ret {
				v = 0
			}
			if v == A {
				cf.Pc++
			}
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_TEST
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			C := int(inst>>9) & 0x1ff //GETC
			if LVAsBool(reg.Get(RA)) == (C == 0) {
				cf.Pc++
			}
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_TESTSET
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff)    //GETB
			C := int(inst>>9) & 0x1ff //GETC
			if value := reg.Get(lbase + B); LVAsBool(value) != (C == 0) {
				// +inline-call reg.Set RA value
			} else {
				cf.Pc++
			}
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_CALL
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff)    //GETB
			C := int(inst>>9) & 0x1ff //GETC
			nargs := B - 1
			if B == 0 {
				nargs = reg.Top() - (RA + 1)
			}
			lv := reg.Get(RA)
			nret := C - 1
			var callable *LFunction
			var meta bool
			if fn, ok := lv.AsLFunction(); ok {
				callable = fn
				meta = false
			} else {
				callable, meta = L.metaCall(lv)
			}

			if callable != nil && callable.IsFast {
				if callFastGFunction(L, callable, RA, nret, false) {
					return retFail
				} else {
					return 0
				}
			}

			// manually inline .pushCallFrame() to avoid duffcopy
			{
				if L.stack.IsFull() {
					L.RaiseError("stack overflow")
				}
				if callable == nil {
					L.RaiseError("attempt to call a non-function object")
				}
				newcf := L.stack.PushEmpty()
				newcf.Fn = callable
				newcf.Pc = 0
				newcf.Base = RA
				newcf.LocalBase = RA + 1
				newcf.ReturnBase = RA
				newcf.NArgs = nargs
				newcf.NRet = nret
				newcf.Parent = cf
				newcf.TailCall = 0
				if meta {
					newcf.NArgs++
					L.reg.Insert(lv, newcf.LocalBase)
				}
				// +inline-call L.initCallFrame newcf
				L.currentFrame = newcf
			}
			if callable.IsG {
				if callGFunction(L, false) {
					return retFail
				} else {
					return 0
				}
			}
			return retCFChanged
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_TAILCALL
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff) //GETB
			nargs := B - 1
			if B == 0 {
				nargs = reg.Top() - (RA + 1)
			}
			lv := reg.Get(RA)
			var callable *LFunction
			var meta bool
			if fn, ok := lv.AsLFunction(); ok {
				callable = fn
				meta = false
			} else {
				callable, meta = L.metaCall(lv)
			}
			if callable == nil {
				L.RaiseError("attempt to call a non-function object")
				return 0
			}
			// +inline-call L.closeUpvalues lbase
			if callable.IsG {
				luaframe := cf
				L.pushCallFrame(callFrame{
					Fn:         callable,
					Pc:         0,
					Base:       RA,
					LocalBase:  RA + 1,
					ReturnBase: cf.ReturnBase,
					NArgs:      nargs,
					NRet:       cf.NRet,
					Parent:     cf,
					TailCall:   0,
				}, lv, meta)
				if callGFunction(L, true) {
					return retFail
				}
				if L.currentFrame == nil || L.currentFrame.Fn.IsG || luaframe == baseframe {
					return retFail
				}
				return retCFChanged
			} else {
				base := cf.Base
				cf.Fn = callable
				cf.Pc = 0
				cf.Base = RA
				cf.LocalBase = RA + 1
				cf.NArgs = nargs
				cf.TailCall++
				lbase := cf.LocalBase
				if meta {
					cf.NArgs++
					L.reg.Insert(lv, cf.LocalBase)
				}
				// +inline-call L.initCallFrame cf
				// +inline-call L.reg.CopyRange base RA -1 reg.Top()-RA-1
				cf.Base = base
				cf.LocalBase = base + (cf.LocalBase - lbase + 1)
				return retFnChanged
			}
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_RETURN
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff) //GETB
			// +inline-call L.closeUpvalues lbase
			nret := B - 1
			if B == 0 {
				nret = reg.Top() - RA
			}
			n := cf.NRet
			if cf.NRet == MultRet {
				n = nret
			}

			if L.Parent != nil && L.stack.Sp() == 1 {
				// +inline-call copyReturnValues L reg.Top() RA n B
				switchToParentThread(L, n, false, true)
				return retFail
			}
			islast := baseframe == L.stack.Pop() || L.stack.IsEmpty()
			// +inline-call copyReturnValues L cf.ReturnBase RA n B
			L.currentFrame = L.stack.Last()
			if islast || L.currentFrame == nil || L.currentFrame.Fn.IsG {
				return retFail
			}
			return retCFChanged
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_FORLOOP
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			if init, ok1 := reg.Get(RA).AsLNumber(); ok1 {
				if limit, ok2 := reg.Get(RA + 1).AsLNumber(); ok2 {
					if step, ok3 := reg.Get(RA + 2).AsLNumber(); ok3 {
						init += step
						v := LNumber(init)
						// +inline-call reg.SetNumber RA v
						if (step > 0 && init <= limit) || (step <= 0 && init >= limit) {
							Sbx := int(inst&0x3ffff) - opMaxArgSbx //GETSBX
							cf.Pc += Sbx
							// +inline-call reg.SetNumber RA+3 v
						} else {
							// +inline-call reg.SetTop RA+1
						}
					} else {
						L.RaiseError("for statement step must be a number")
					}
				} else {
					L.RaiseError("for statement limit must be a number")
				}
			} else {
				L.RaiseError("for statement init must be a number")
			}
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_FORPREP
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			Sbx := int(inst&0x3ffff) - opMaxArgSbx //GETSBX
			if init, ok1 := reg.Get(RA).AsLNumber(); ok1 {
				if step, ok2 := reg.Get(RA + 2).AsLNumber(); ok2 {
					// +inline-call reg.SetNumber RA LNumber(init-step)
				} else {
					L.RaiseError("for statement step must be a number")
				}
			} else {
				L.RaiseError("for statement init must be a number")
			}
			cf.Pc += Sbx
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_TFORLOOP
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			C := int(inst>>9) & 0x1ff //GETC
			nret := C
			// +inline-call reg.SetTop RA+3+2
			// +inline-call reg.Set RA+3+2 reg.Get(RA+2)
			// +inline-call reg.Set RA+3+1 reg.Get(RA+1)
			// +inline-call reg.Set RA+3 reg.Get(RA)
			L.callR(2, nret, RA+3)
			if value := reg.Get(RA + 3); !value.EqualsLNil() {
				// +inline-call reg.Set RA+2 value
				pc := cf.Fn.Proto.Code[cf.Pc]
				cf.Pc += int(pc&0x3ffff) - opMaxArgSbx
			}
			cf.Pc++
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_SETLIST
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff)    //GETB
			C := int(inst>>9) & 0x1ff //GETC
			if C == 0 {
				C = int(cf.Fn.Proto.Code[cf.Pc])
				cf.Pc++
			}
			offset := (C - 1) * FieldsPerFlush
			table := reg.Get(RA).MustLTable()
			nelem := B
			if B == 0 {
				nelem = reg.Top() - RA - 1
			}
			for i := 1; i <= nelem; i++ {
				table.RawSetInt(offset+i, reg.Get(RA+i))
			}
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_CLOSE
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			// +inline-call L.closeUpvalues RA
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_CLOSURE
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			Bx := int(inst & 0x3ffff) //GETBX
			proto := cf.Fn.Proto.FunctionPrototypes[Bx]
			closure := newLFunctionL(proto, cf.Fn.Env, int(proto.NumUpvalues))
			// +inline-call reg.Set RA closure
			for i := 0; i < int(proto.NumUpvalues); i++ {
				inst = cf.Fn.Proto.Code[cf.Pc]
				cf.Pc++
				B := opGetArgB(inst)
				switch opGetOpCode(inst) {
				case OP_MOVE:
					closure.Upvalues[i] = L.findUpvalue(lbase + B)
				case OP_GETUPVAL:
					closure.Upvalues[i] = cf.Fn.Upvalues[B]
				}
			}
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_VARARG
			reg := L.reg
			cf := L.currentFrame
			lbase := cf.LocalBase
			A := int(inst>>18) & 0xff //GETA
			RA := lbase + A
			B := int(inst & 0x1ff) //GETB
			nparams := int(cf.Fn.Proto.NumParameters)
			nvarargs := cf.NArgs - nparams
			if nvarargs < 0 {
				nvarargs = 0
			}
			nwant := B - 1
			if B == 0 {
				nwant = nvarargs
			}
			// +inline-call reg.CopyRange RA cf.Base+nparams+1 cf.LocalBase nwant
			return 0
		},
		func(L *LState, inst uint32, baseframe *callFrame) int { //OP_NOP
			return 0
		},
	}
}

func opArith(L *LState, inst uint32, baseframe *callFrame) int { //OP_ADD, OP_SUB, OP_MUL, OP_DIV, OP_MOD, OP_POW
	reg := L.reg
	cf := L.currentFrame
	lbase := cf.LocalBase
	A := int(inst>>18) & 0xff //GETA
	RA := lbase + A
	opcode := int(inst >> 26) //GETOPCODE
	B := int(inst & 0x1ff)    //GETB
	C := int(inst>>9) & 0x1ff //GETC
	lhs := L.rkValue(B)
	rhs := L.rkValue(C)
	v1, ok1 := lhs.AsLNumber()
	v2, ok2 := rhs.AsLNumber()
	if ok1 && ok2 {
		v := numberArith(L, opcode, LNumber(v1), LNumber(v2))
		// +inline-call reg.SetNumber RA v
	} else {
		v := objectArith(L, opcode, lhs, rhs)
		// +inline-call reg.Set RA v
	}
	return 0
}

func luaModulo(lhs, rhs LNumber) LNumber {
	flhs := float64(lhs)
	frhs := float64(rhs)
	v := math.Mod(flhs, frhs)
	if frhs > 0 && v < 0 || frhs < 0 && v > 0 {
		v += frhs
	}
	return LNumber(v)
}

func numberArith(L *LState, opcode int, lhs, rhs LNumber) LNumber {
	switch opcode {
	case OP_ADD:
		return lhs + rhs
	case OP_SUB:
		return lhs - rhs
	case OP_MUL:
		return lhs * rhs
	case OP_DIV:
		return lhs / rhs
	case OP_MOD:
		return luaModulo(lhs, rhs)
	case OP_POW:
		flhs := float64(lhs)
		frhs := float64(rhs)
		return LNumber(math.Pow(flhs, frhs))
	}
	panic("should not reach here")
}

func objectArith(L *LState, opcode int, lhs, rhs LValue) LValue {
	event := ""
	switch opcode {
	case OP_ADD:
		event = "__add"
	case OP_SUB:
		event = "__sub"
	case OP_MUL:
		event = "__mul"
	case OP_DIV:
		event = "__div"
	case OP_MOD:
		event = "__mod"
	case OP_POW:
		event = "__pow"
	}
	op := L.metaOp2(lhs, rhs, event)
	if _, ok := op.AsLFunction(); ok {
		L.reg.Push(op)
		L.reg.Push(lhs)
		L.reg.Push(rhs)
		L.Call(2, 1)
		return L.reg.Pop()
	}
	if str, ok := lhs.AsLString(); ok {
		if lnum, err := parseNumber(string(str)); err == nil {
			lhs = lnum.AsLValue()
		}
	}
	if str, ok := rhs.AsLString(); ok {
		if rnum, err := parseNumber(string(str)); err == nil {
			rhs = rnum.AsLValue()
		}
	}
	if v1, ok1 := lhs.AsLNumber(); ok1 {
		if v2, ok2 := rhs.AsLNumber(); ok2 {
			return numberArith(L, opcode, LNumber(v1), LNumber(v2)).AsLValue()
		}
	}
	L.RaiseError(fmt.Sprintf("cannot perform %v operation between %v and %v",
		strings.TrimLeft(event, "_"), lhs.Type().String(), rhs.Type().String()))

	return LValue{}
}

func stringConcat(L *LState, total, last int) LValue {
	rhs := L.reg.Get(last)
	total--
	for i := last - 1; total > 0; {
		lhs := L.reg.Get(i)
		if !(LVCanConvToString(lhs) && LVCanConvToString(rhs)) {
			op := L.metaOp2(lhs, rhs, "__concat")
			if op.Type() == LTFunction {
				L.reg.Push(op)
				L.reg.Push(lhs)
				L.reg.Push(rhs)
				L.Call(2, 1)
				rhs = L.reg.Pop()
				total--
				i--
			} else {
				L.RaiseError("cannot perform concat operation between %v and %v", lhs.Type().String(), rhs.Type().String())
				return LValue{}
			}
		} else {
			buf := make([]string, total+1)
			buf[total] = LVAsString(rhs)
			for total > 0 {
				lhs = L.reg.Get(i)
				if !LVCanConvToString(lhs) {
					break
				}
				buf[total-1] = LVAsString(lhs)
				i--
				total--
			}
			rhs = LString(strings.Join(buf, "")).AsLValue()
		}
	}
	return rhs
}

func lessThan(L *LState, lhs, rhs LValue) bool {
	// optimization for numbers
	if v1, ok1 := lhs.AsLNumber(); ok1 {
		if v2, ok2 := rhs.AsLNumber(); ok2 {
			return v1 < v2
		}
		L.RaiseError("attempt to compare %v with %v", lhs.Type().String(), rhs.Type().String())
	}
	if lhs.Type() != rhs.Type() {
		L.RaiseError("attempt to compare %v with %v", lhs.Type().String(), rhs.Type().String())
		return false
	}
	ret := false
	switch lhs.Type() {
	case LTString:
		ret = strCmp(string(lhs.MustLString()), string(rhs.MustLString())) < 0
	default:
		ret = objectRationalWithError(L, lhs, rhs, "__lt")
	}
	return ret
}

func equals(L *LState, lhs, rhs LValue, raw bool) bool {
	lt := lhs.Type()
	if lt != rhs.Type() {
		return false
	}

	ret := false
	switch lt {
	case LTNil:
		ret = true
	case LTNumber:
		v1, _ := lhs.AsLNumber()
		v2, _ := rhs.AsLNumber()
		ret = v1 == v2
	case LTBool:
		ret = bool(lhs.MustLBool()) == bool(rhs.MustLBool())
	case LTString:
		ret = string(lhs.MustLString()) == string(rhs.MustLString())
	case LTUserData, LTTable:
		if lhs.Equals(rhs) {
			ret = true
		} else if !raw {
			switch objectRational(L, lhs, rhs, "__eq") {
			case 1:
				ret = true
			default:
				ret = false
			}
		}
	default:
		ret = lhs.Equals(rhs)
	}
	return ret
}

func objectRationalWithError(L *LState, lhs, rhs LValue, event string) bool {
	switch objectRational(L, lhs, rhs, event) {
	case 1:
		return true
	case 0:
		return false
	}
	L.RaiseError("attempt to compare %v with %v", lhs.Type().String(), rhs.Type().String())
	return false
}

func objectRational(L *LState, lhs, rhs LValue, event string) int {
	m1 := L.metaOp1(lhs, event)
	m2 := L.metaOp1(rhs, event)
	if m1.Type() == LTFunction && m1.Equals(m2) {
		L.reg.Push(m1)
		L.reg.Push(lhs)
		L.reg.Push(rhs)
		L.Call(2, 1)
		if LVAsBool(L.reg.Pop()) {
			return 1
		}
		return 0
	}
	return -1
}
