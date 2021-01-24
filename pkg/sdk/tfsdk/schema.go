package tfsdk

type Schema struct {
	KeyAWSRegion     string
	KeyAWSProfile    string
	KeyAWSAssumeRole string
}

func defaultSchema() *Schema {
	return &Schema{
		KeyAWSRegion:     KeyRegion,
		KeyAWSProfile:    KeyProfile,
		KeyAWSAssumeRole: KeyAssumeRole,
	}
}

type SchemaOption interface {
	Apply(*Schema)
}

func (s *Schema) Apply(other *Schema) {
	*other = *s
}

type schemaOptionFunc struct {
	f func(*Schema)
}

func (f *schemaOptionFunc) Apply(schema *Schema) {
	f.f(schema)
}

func SchemaOptionFunc(f func(*Schema)) SchemaOption {
	return &schemaOptionFunc{
		f: f,
	}
}

func SchemaOptionAWSRegionKey(k string) SchemaOption {
	return SchemaOptionFunc(func(schema *Schema) {
		schema.KeyAWSRegion = k
	})
}

func SchemaOptionAWSProfileKey(k string) SchemaOption {
	return SchemaOptionFunc(func(schema *Schema) {
		schema.KeyAWSProfile = k
	})
}

func SchemaOptionAWSAssumeRole(k string) SchemaOption {
	return SchemaOptionFunc(func(schema *Schema) {
		schema.KeyAWSAssumeRole = k
	})
}

func CreateSchema(opts ...SchemaOption) *Schema {
	schema := defaultSchema()

	for _, o := range opts {
		o.Apply(schema)
	}

	return schema
}
