/*
 * Tsuru
 *
 * Open source, extensible and Docker-based Platform as a Service (PaaS)
 *
 * API version: 1.6
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */

package tsuru

type NodeUpdateData struct {
	Address string `json:"address,omitempty"`

	Pool string `json:"pool,omitempty"`

	Metadata map[string]string `json:"metadata,omitempty"`

	Enable bool `json:"enable,omitempty"`

	Disable bool `json:"disable,omitempty"`
}
