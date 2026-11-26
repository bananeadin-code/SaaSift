package filter

type Filter struct {
	Codes map[int]bool
	Sizes map[int]bool
	Words map[int]bool
}

func (f Filter) Match(status, size, words int) bool {
	return f.Codes[status] || f.Sizes[size] || f.Words[words]
}
