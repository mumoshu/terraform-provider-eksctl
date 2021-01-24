package tfsdk

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/api"
	"log"
)

func GetAssumeRoleConfig(d api.Getter, opts ...SchemaOption) (config *sdk.AssumeRoleConfig) {
	sc := CreateSchema(opts...)

	if l, ok := d.Get(sc.KeyAWSAssumeRole).([]interface{}); ok && len(l) > 0 && l[0] != nil {
		config = &sdk.AssumeRoleConfig{}

		m := l[0].(map[string]interface{})

		if v, ok := m["duration_seconds"].(int); ok && v != 0 {
			config.DurationSeconds = int64(v)
		}

		if v, ok := m["external_id"].(string); ok && v != "" {
			config.ExternalID = v
		}

		if v, ok := m["policy"].(string); ok && v != "" {
			config.Policy = v
		}

		if policyARNSet, ok := m["policy_arns"].(*schema.Set); ok && policyARNSet.Len() > 0 {
			for _, policyARNRaw := range policyARNSet.List() {
				policyARN, ok := policyARNRaw.(string)

				if !ok {
					continue
				}

				config.PolicyARNs = append(config.PolicyARNs, policyARN)
			}
		}

		if v, ok := m["role_arn"].(string); ok && v != "" {
			config.RoleARN = v
		}

		if v, ok := m["session_name"].(string); ok && v != "" {
			config.SessionName = v
		}

		if tagMapRaw, ok := m["tags"].(map[string]interface{}); ok && len(tagMapRaw) > 0 {
			config.Tags = make(map[string]string)

			for k, vRaw := range tagMapRaw {
				v, ok := vRaw.(string)

				if !ok {
					continue
				}

				config.Tags[k] = v
			}
		}

		if transitiveTagKeySet, ok := m["transitive_tag_keys"].(*schema.Set); ok && transitiveTagKeySet.Len() > 0 {
			for _, transitiveTagKeyRaw := range transitiveTagKeySet.List() {
				transitiveTagKey, ok := transitiveTagKeyRaw.(string)

				if !ok {
					continue
				}

				config.TransitiveTagKeys = append(config.TransitiveTagKeys, transitiveTagKey)
			}
		}

		log.Printf("[INFO] assume_role configuration set: (ARN: %q, SessionID: %q, ExternalID: %q)", config.RoleARN, config.SessionName, config.ExternalID)
	}

	return
}
