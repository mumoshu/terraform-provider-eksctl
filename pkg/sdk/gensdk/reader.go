package gensdk

type MapReader struct {
	M map[string]interface{}
}

func (r *MapReader) Get(k string) interface{} {
	return r.M[k]
}

func (r *MapReader) List(k string) []interface{} {
	if l, ok := r.M[k].([]interface{}); ok {
		return l
	}

	return nil
}

