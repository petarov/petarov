package helper

type AnySet[K comparable] struct {
	elements map[K]struct{}
}

func NewAnySet[K comparable]() *AnySet[K] {
	return &AnySet[K]{elements: make(map[K]struct{})}
}

func (s *AnySet[K]) Add(element K) {
	s.elements[element] = struct{}{}
}

func (s *AnySet[K]) Remove(element K) {
	delete(s.elements, element)
}

func (s *AnySet[K]) Contains(element K) bool {
	_, exists := s.elements[element]
	return exists
}
