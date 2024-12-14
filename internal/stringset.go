package helper

type StringSet struct {
	elements map[string]struct{}
}

func NewStringSet() *StringSet {
	return &StringSet{elements: make(map[string]struct{})}
}

func (s *StringSet) Add(element string) {
	s.elements[element] = struct{}{}
}

func (s *StringSet) Remove(element string) {
	delete(s.elements, element)
}

func (s *StringSet) Contains(element string) bool {
	_, exists := s.elements[element]
	return exists
}
