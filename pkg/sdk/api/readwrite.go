package api

type ReadWrite interface {
	Getter

	Id() string

	Set(string, interface{}) error
}
