// This Source Code Form is subject to the terms of the Mozilla Public License,
// v. 2.0. If a copy of the MPL was not distributed with this file, You can
// obtain one at http://mozilla.org/MPL/2.0/
//
// Copyright (C) HashiCorp, Inc.
//
// This is a modified and enhanced version of the original source code.
//
// The original source code contained in this file can be found at
// https://github.com/hashicorp/terraform-provider-aws/blob/bbefb8dfad459bc1846038b1da4c5857afe55bd9/aws/provider.go#L1314
package sdk

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/api"
	"log"
)

func GetAssumeRoleConfig(d api.Getter) (config *AssumeRoleConfig) {
	if l, ok := d.Get(KeyAssumeRole).([]interface{}); ok && len(l) > 0 && l[0] != nil {
		config = &AssumeRoleConfig{}

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
