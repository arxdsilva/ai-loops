package tun

const a = 1

func f() {}

var (
	b, c = 1, 2
)

type I interface{ M() }

type T struct{}

func (*T) M() {}

var _ I = (*T)(nil)
