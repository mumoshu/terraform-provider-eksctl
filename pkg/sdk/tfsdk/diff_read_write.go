package tfsdk

import "github.com/hashicorp/terraform-plugin-sdk/helper/schema"

type DiffReadWrite struct {
	D *schema.ResourceDiff
}

func (d *DiffReadWrite) Get(k string) interface{} {
	return d.D.Get(k)
}

func (d *DiffReadWrite) List(k string) []interface{} {
	return nil
}

func (d *DiffReadWrite) Set(k string, v interface{}) error {
	return d.D.SetNew(k, v)
}

func (d *DiffReadWrite) SetNewComputed(k string) error {
	return d.D.SetNewComputed(k)
}

func (d *DiffReadWrite) Id() string {
	return d.D.Id()
}
