#!/usr/bin/env python

import subprocess
import re

regex = re.compile(
    r"(?P<file>\./.+\.go):(?P<line>\d+):(?P<col>\d+): cannot use (?P<expr>[\s\S]+) \([^\(]+ of type .+\) as LValue value",
    re.M,
)


def staticcheck():
    p = subprocess.Popen(["staticcheck", "."], stdout=subprocess.PIPE)
    p.wait()
    diag = p.stdout.read().decode("utf-8")
    return diag


def repl(after, expr):
    j = 0
    i = 0
    while i < len(expr) and j < len(after):
        if after[j] == expr[i]:
            i += 1
            j += 1
            continue
        if after[j] == " ":
            j += 1
            continue
        if expr[i] == " ":
            i += 1
            continue
        print("Something wrong")
        import sys

        sys.exit(1)
    return after[:j] + ".AsLValue()" + after[j:]


while True:
    diag = staticcheck()
    g = None
    for line in diag.splitlines():
        g = regex.match(line)
        if g is not None:
            break
    print(diag)
    if g is None:
        print("All done")
        break
    contents = open(g.group("file")).read()
    lines = contents.splitlines()
    lineno = int(g.group("line"))
    col = int(g.group("col")) - 1
    line = lines[lineno - 1]
    before = line[:col]
    after = line[col:]
    expr = g.group("expr")
    after = repl(after, expr)
    # if after.startswith(expr):
    #     after = expr + ".AsLValue()" + after[len(expr) :]
    # else:
    #     print("something wrong")
    #     print(g.groups())
    #     print(line)
    #     break
    line = before + after
    print("Fixed: ", line)
    lines[lineno - 1] = line
    open(g.group("file"), "w").write("\n".join(lines))
