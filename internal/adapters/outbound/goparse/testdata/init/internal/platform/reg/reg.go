package reg

// T is a type with a method named init, which is not an init func.
type T struct{}

func (T) init() {}

func init() {
	_ = T{}
}
