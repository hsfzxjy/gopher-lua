package main

import (
	"fmt"
	"go/token"
	"log"
	"os"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func doFunc(p *ssa.Program, f *ssa.Function) {
	for _, f := range f.AnonFuncs {
		doFunc(p, f)
	}
	for _, b := range f.Blocks {
		doBlock(p, f, b)
	}
}

func posFor(i *ssa.MakeInterface) token.Pos {
	if i.Pos() != token.NoPos {
		return i.Pos()
	}
	for _, r := range *i.Referrers() {
		if r.Pos() != token.NoPos {
			return r.Pos()
		}
	}
	return 0
}

func doBlock(prog *ssa.Program, f *ssa.Function, b *ssa.BasicBlock) {
	for _, instr := range b.Instrs {
		switch instr := instr.(type) {
		case *ssa.MakeInterface:
			_ = instr
			if instr.X.Type().String() == "github.com/hsfzxjy/gopher-lua.LValue" {
				println(instr.Type().String())
				fmt.Println(prog.Fset.Position(posFor(instr)))
			}
		default:

		}
	}
}

func main() {
	// Load, parse, and type-check the initial packages.
	cfg := &packages.Config{Mode: packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedTypesSizes | packages.NeedDeps | packages.NeedImports}
	initial, err := packages.Load(cfg, os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	// Stop if any package had errors.
	// This step is optional; without it, the next step
	// will create SSA for only a subset of packages.
	if packages.PrintErrors(initial) > 0 {
		log.Fatalf("packages contain errors")
	}

	// Create SSA packages for all well-typed packages.
	prog, pkgs := ssautil.Packages(initial, 0)
	prog.Build()
	for _, p := range pkgs {
		for name, f := range p.Members {
			_ = name
			f, ok := f.(*ssa.Function)
			if !ok {
				continue
			}
			doFunc(prog, f)
		}
	}
}
