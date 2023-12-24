package lua

import (
	"os"
	"strings"
	"time"
)

var startedAt time.Time

func init() {
	startedAt = time.Now()
}

func getIntField(L *LState, tb *LTable, key string, v int) int {
	ret := tb.RawGetString(key)

	switch lv := ret; lv.Type() {
	case LTNumber:
		return int(lv.MustLNumber())
	case LTString:
		slv := string(lv.MustLString())
		slv = strings.TrimLeft(slv, " ")
		if strings.HasPrefix(slv, "0") && !strings.HasPrefix(slv, "0x") && !strings.HasPrefix(slv, "0X") {
			// Standard lua interpreter only support decimal and hexadecimal
			slv = strings.TrimLeft(slv, "0")
			if slv == "" {
				return 0
			}
		}
		if num, err := parseNumber(slv); err == nil {
			return int(num)
		}
	default:
		return v
	}

	return v
}

func getBoolField(L *LState, tb *LTable, key string, v bool) bool {
	ret := tb.RawGetString(key)
	if lb, ok := ret.AsLBool(); ok {
		return bool(lb)
	}
	return v
}

func OpenOs(L *LState) int {
	osmod := L.RegisterModule(OsLibName, osFuncs)
	L.Push(osmod)
	return 1
}

var osFuncs = map[string]LGFunction{
	"clock":     osClock,
	"difftime":  osDiffTime,
	"execute":   osExecute,
	"exit":      osExit,
	"date":      osDate,
	"getenv":    osGetEnv,
	"remove":    osRemove,
	"rename":    osRename,
	"setenv":    osSetEnv,
	"setlocale": osSetLocale,
	"time":      osTime,
	"tmpname":   osTmpname,
}

func osClock(L *LState) int {
	L.Push(LNumber(float64(time.Since(startedAt)) / float64(time.Second)).AsLValue())
	return 1
}

func osDiffTime(L *LState) int {
	L.Push(LNumber(L.CheckInt64(1) - L.CheckInt64(2)).AsLValue())
	return 1
}

func osExecute(L *LState) int {
	var procAttr os.ProcAttr
	procAttr.Files = []*os.File{os.Stdin, os.Stdout, os.Stderr}
	cmd, args := popenArgs(L.CheckString(1))
	args = append([]string{cmd}, args...)
	process, err := os.StartProcess(cmd, args, &procAttr)
	if err != nil {
		L.Push(LNumber(1).AsLValue())
		return 1
	}

	ps, err := process.Wait()
	if err != nil || !ps.Success() {
		L.Push(LNumber(1).AsLValue())
		return 1
	}
	L.Push(LNumber(0).AsLValue())
	return 1
}

func osExit(L *LState) int {
	L.Close()
	os.Exit(L.OptInt(1, 0))
	return 1
}

func osDate(L *LState) int {
	t := time.Now()
	isUTC := false
	cfmt := "%c"
	if L.GetTop() >= 1 {
		cfmt = L.CheckString(1)
		if strings.HasPrefix(cfmt, "!") {
			cfmt = strings.TrimLeft(cfmt, "!")
			isUTC = true
		}
		if L.GetTop() >= 2 {
			t = time.Unix(L.CheckInt64(2), 0)
		}
		if isUTC {
			t = t.UTC()
		}
		if strings.HasPrefix(cfmt, "*t") {
			ret := L.NewTable()
			ret.RawSetString("year", LNumber(t.Year()).AsLValue())
			ret.RawSetString("month", LNumber(t.Month()).AsLValue())
			ret.RawSetString("day", LNumber(t.Day()).AsLValue())
			ret.RawSetString("hour", LNumber(t.Hour()).AsLValue())
			ret.RawSetString("min", LNumber(t.Minute()).AsLValue())
			ret.RawSetString("sec", LNumber(t.Second()).AsLValue())
			ret.RawSetString("wday", LNumber(t.Weekday()+1).AsLValue())
			// TODO yday & dst
			ret.RawSetString("yday", LNumber(0).AsLValue())
			ret.RawSetString("isdst", LFalse.AsLValue())
			L.Push(ret.AsLValue())
			return 1
		}
	}
	L.Push(LString(strftime(t, cfmt)).AsLValue())
	return 1
}

func osGetEnv(L *LState) int {
	v := os.Getenv(L.CheckString(1))
	if len(v) == 0 {
		L.Push(LNil)
	} else {
		L.Push(LString(v).AsLValue())
	}
	return 1
}

func osRemove(L *LState) int {
	err := os.Remove(L.CheckString(1))
	if err != nil {
		L.Push(LNil)
		L.Push(LString(err.Error()).AsLValue())
		return 2
	} else {
		L.Push(LTrue.AsLValue())
		return 1
	}
}

func osRename(L *LState) int {
	err := os.Rename(L.CheckString(1), L.CheckString(2))
	if err != nil {
		L.Push(LNil)
		L.Push(LString(err.Error()).AsLValue())
		return 2
	} else {
		L.Push(LTrue.AsLValue())
		return 1
	}
}

func osSetLocale(L *LState) int {
	// setlocale is not supported
	L.Push(LFalse.AsLValue())
	return 1
}

func osSetEnv(L *LState) int {
	err := os.Setenv(L.CheckString(1), L.CheckString(2))
	if err != nil {
		L.Push(LNil)
		L.Push(LString(err.Error()).AsLValue())
		return 2
	} else {
		L.Push(LTrue.AsLValue())
		return 1
	}
}

func osTime(L *LState) int {
	if L.GetTop() == 0 {
		L.Push(LNumber(time.Now().Unix()).AsLValue())
	} else {
		lv := L.CheckAny(1)
		if lv == LNil {
			L.Push(LNumber(time.Now().Unix()).AsLValue())
		} else {
			tbl, ok := lv.AsLTable()
			if !ok {
				L.TypeError(1, LTTable)
			}
			sec := getIntField(L, tbl, "sec", 0)
			min := getIntField(L, tbl, "min", 0)
			hour := getIntField(L, tbl, "hour", 12)
			day := getIntField(L, tbl, "day", -1)
			month := getIntField(L, tbl, "month", -1)
			year := getIntField(L, tbl, "year", -1)
			isdst := getBoolField(L, tbl, "isdst", false)
			t := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.Local)
			// TODO dst
			if false {
				print(isdst)
			}
			L.Push(LNumber(t.Unix()).AsLValue())
		}
	}
	return 1
}

func osTmpname(L *LState) int {
	file, err := os.CreateTemp("", "")
	if err != nil {
		L.RaiseError("unable to generate a unique filename")
	}
	file.Close()
	os.Remove(file.Name()) // ignore errors
	L.Push(LString(file.Name()).AsLValue())
	return 1
}

//
