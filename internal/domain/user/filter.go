package user

// ListFilter contém os parâmetros para filtrar a listagem de users.
type ListFilter struct {
	Page       int
	Limit      int
	Name       string
	Email      string
	ActiveOnly bool
}

// Normalize aplica valores padrão aos parâmetros de paginação.
func (f *ListFilter) Normalize() {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit < 1 || f.Limit > 100 {
		f.Limit = 20
	}
}

// Offset calcula o offset para a query SQL.
func (f *ListFilter) Offset() int {
	return (f.Page - 1) * f.Limit
}

// ListResult contém o resultado da listagem paginada.
type ListResult struct {
	Users []*User
	Total int
	Page  int
	Limit int
}
