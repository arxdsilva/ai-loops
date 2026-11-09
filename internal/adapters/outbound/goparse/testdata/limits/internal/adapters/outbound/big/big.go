package big

// Big is a god type.
type Big struct {
	F1  int
	F2  int
	F3  int
	F4  int
	F5  int
	F6  int
	F7  int
	F8  int
	F9  int
	F10 int
	F11 int
	F12 int
	F13 int
	F14 int
	F15 int
	F16 int
	F17 int
	F18 int
	F19 int
	F20 int
	F21 int
	F22 int
	F23 int
	F24 int
	F25 int
	F26 int
}

func (b *Big) M1() int { return b.F1 }

func (b *Big) M2() int { return b.F1 }

func (b *Big) M3() int { return b.F1 }

func (b *Big) M4() int { return b.F1 }

func (b *Big) M5() int { return b.F1 }

func (b *Big) M6() int { return b.F1 }

func (b *Big) M7() int { return b.F1 }

func (b *Big) M8() int { return b.F1 }

func (b *Big) M9() int { return b.F1 }

func (b *Big) M10() int { return b.F1 }

func (b *Big) M11() int { return b.F1 }

func (b *Big) M12() int { return b.F1 }

func (b *Big) M13() int { return b.F1 }

func (b *Big) M14() int { return b.F1 }

func (b *Big) M15() int { return b.F1 }

func (b *Big) M16() int { return b.F1 }

func (b *Big) M17() int { return b.F1 }

func (b *Big) M18() int { return b.F1 }

func (b *Big) M19() int { return b.F1 }

func (b *Big) M20() int { return b.F1 }

func (b *Big) M21() int { return b.F1 }

func (b *Big) M22() int { return b.F1 }

func (b *Big) M23() int { return b.F1 }

func (b *Big) M24() int { return b.F1 }

func (b *Big) M25() int { return b.F1 }

func (b *Big) M26() int { return b.F1 }

func (b *Big) M27() int { return b.F1 }

func (b *Big) M28() int { return b.F1 }

func (b *Big) M29() int { return b.F1 }

func (b *Big) M30() int { return b.F1 }

func (b *Big) M31() int { return b.F1 }

func (b *Big) M32() int { return b.F1 }

func (b *Big) M33() int { return b.F1 }

func (b *Big) M34() int { return b.F1 }

func (b *Big) M35() int { return b.F1 }

func (b *Big) M36() int { return b.F1 }

func (b *Big) M37() int { return b.F1 }

func (b *Big) M38() int { return b.F1 }

func (b *Big) M39() int { return b.F1 }

func (b *Big) M40() int { return b.F1 }

func (b *Big) M41() int { return b.F1 }

// Small is fine.
type Small interface {
	A()
}

// Fat has eight methods and an embedded interface.
type Fat interface {
	Small
	I1()
	I2()
	I3()
	I4()
	I5()
	I6()
	I7()
	I8()
}
