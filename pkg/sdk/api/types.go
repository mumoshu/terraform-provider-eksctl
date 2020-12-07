package api

type Getter interface {
	Get(string) interface{}
}

type Lister interface {
	Getter

	List(string) []interface{}
}

type UniqueResourceGetter interface {
	Id() string

	Getter
}

type Resource interface {
	Set(string, interface{}) error

	UniqueResourceGetter
}
