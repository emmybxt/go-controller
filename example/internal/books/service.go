package books

type Book struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

type Service struct{}

func NewService() *Service { return &Service{} }

func (s *Service) Find(id string) (Book, bool) {
	if id != "1" {
		return Book{}, false
	}
	return Book{ID: "1", Title: "The Go Programming Language"}, true
}
