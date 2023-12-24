#!/usr/bin/env python

import sys
import re

files = sys.argv
regex_two = re.compile(
    r"(?P<var>\w+)\s*,\s*(?P<ok>\w+)\s*(?P<colon>:?)=(?P<expr>.+)\.\((?P<typ>LNumber|LString|LBool|\*LState|\*LFunction|\*LNilType|\*LUserData|LChannel|\*LTable)\)"
)
regex_one = re.compile(
    r"\.\((?P<typ>LNumber|LString|LBool|\*LState|\*LFunction|\*LNilType|\*LUserData|LChannel|\*LTable)\)"
)


def repl_two(g: re.Match) -> str:
    typ = g.group("typ").replace("*", "")
    return f"{g.group('var')}, {g.group('ok')} {g.group('colon')}={g.group('expr')}.As{typ}()"


def repl_one(g: re.Match) -> str:
    print(g)
    typ = g.group("typ").replace("*", "")
    return f".Must{typ}()"


for file in files:
    contents = open(file).read()
    contents = regex_two.sub(repl_two, contents)
    contents = regex_one.sub(repl_one, contents)
    open(file, "w").write(contents)
