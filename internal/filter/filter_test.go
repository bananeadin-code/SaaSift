package filter

import "testing"

func TestMatch(t *testing.T) {
	f := Filter{
		Codes: map[int]bool{404: true, 403: true},
		Sizes: map[int]bool{111: true},
		Words: map[int]bool{672: true},
	}

	tests := []struct {
		name               string
		status, size, word int
		want               bool
	}{
		{"código filtrado", 404, 50, 3, true},
		{"tamaño filtrado", 200, 111, 3, true},
		{"palabras filtradas", 200, 7650, 672, true},
		{"nada coincide", 200, 293, 34, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := f.Match(tt.status, tt.size, tt.word); got != tt.want {
				t.Errorf("Match(%d, %d, %d) = %v, esperaba %v",
					tt.status, tt.size, tt.word, got, tt.want)
			}
		})
	}
}

func TestMatch_EmptyFilter(t *testing.T) {
	var f Filter
	if f.Match(404, 0, 0) {
		t.Error("un filtro vacío no debería ocultar nada")
	}
}
