package mastodon

type List struct {
	Name string
}

func NewList() *List {
	l := List{}
	return &l
}
