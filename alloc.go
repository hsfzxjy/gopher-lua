package lua

// allocator is a fast bulk memory allocator for the LValue.
//
// Deprecated: No need anymore due to LValue optimization.
type allocator struct {
}

func newAllocator(size int) *allocator {
	return nil
}

// LNumber2I takes a number value and returns an interface LValue representing the same number.
// Converting an LNumber to a LValue naively, by doing:
// `var val LValue = myLNumber`
// will result in an individual heap alloc of 8 bytes for the float value. LNumber2I amortizes the cost and memory
// overhead of these allocs by allocating blocks of floats instead.
// The downside of this is that all of the floats on a given block have to become eligible for gc before the block
// as a whole can be gc-ed.
//
// Deprecated: No need anymore due to LValue optimization.
func (al *allocator) LNumber2I(v LNumber) LValue {
	return v.AsLValue()
}
