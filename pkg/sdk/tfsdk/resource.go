package tfsdk

import "github.com/hashicorp/terraform-plugin-sdk/helper/schema"

type Resource struct {
	*schema.ResourceData
}

func (r *Resource) List(k string) []interface{} {
	if s := r.Get(k).(*schema.Set); s != nil {
		return s.List()
	}

	return nil
}
